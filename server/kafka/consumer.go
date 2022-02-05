/*
 * Copyright 2020 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package kafka

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/riferrei/srclient"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Message represents a Kafka message.
type Message struct {
	Topic     string
	Partition int
	Offset    int64

	Key     []byte
	Value   []byte
	Headers []sarama.RecordHeader
}

// Consumer represents a Kafka Consumer.
type Consumer interface {
	Fetch(context.Context) (Message, error)
	Commit(context.Context, Message) error
	GroupMode() bool
	Close() error
}

type saramaConsumer struct {
	groupMode bool
	topic     string

	saslOn        bool
	tlsOn         bool
	tlsSkipVerify bool

	c  sarama.Consumer
	pc sarama.PartitionConsumer

	cg           sarama.ConsumerGroup
	fetchCh      chan *sarama.ConsumerMessage
	commitCh     chan *sarama.ConsumerMessage
	consumeErrCh chan error

	cancel context.CancelFunc

	schemaRegistryOn     bool
	schemaRegistryClient srclient.ISchemaRegistryClient
	schemaType           srclient.SchemaType
	pbDeserializer       pbDeserializer
}

// NewConsumer returns a new Kafka Consumer.
func NewConsumer(cc conf.ConnectorConfig, dialTimeout time.Duration) (Consumer, error) {
	sc := sarama.NewConfig()
	sc.Net.DialTimeout = dialTimeout
	sc.ClientID = "nats-kafka-consumer"

	if cc.SASL.User != "" {
		sc.Net.SASL.Enable = true
		sc.Net.SASL.Handshake = true
		sc.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		sc.Net.SASL.User = cc.SASL.User
		sc.Net.SASL.Password = cc.SASL.Password
	}

	if sc.Net.SASL.Enable && cc.SASL.InsecureSkipVerify {
		sc.Net.TLS.Enable = true
		sc.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: cc.SASL.InsecureSkipVerify,
		}
	} else if tlsC, err := cc.TLS.MakeTLSConfig(); tlsC != nil && err == nil {
		sc.Net.TLS.Enable = true
		sc.Net.TLS.Config = tlsC
	}

	if cc.MinBytes > 0 {
		sc.Consumer.Fetch.Min = int32(cc.MinBytes)
	}
	if cc.MaxBytes > 0 {
		sc.Consumer.Fetch.Max = int32(cc.MaxBytes)
	}

	cons := &saramaConsumer{
		groupMode:     cc.GroupID != "",
		topic:         cc.Topic,
		saslOn:        sc.Net.SASL.Enable,
		tlsOn:         sc.Net.TLS.Enable,
		tlsSkipVerify: cc.SASL.InsecureSkipVerify,
	}

	// If schema registry url and subject name both are set, enable schema registry integration
	if cc.SchemaRegistryURL != "" && cc.SubjectName != "" {
		cons.schemaRegistryClient = srclient.CreateSchemaRegistryClient(cc.SchemaRegistryURL)

		switch strings.ToUpper(cc.SchemaType) {
		case srclient.Json.String():
			cons.schemaType = srclient.Json
		case srclient.Protobuf.String():
			cons.schemaType = srclient.Protobuf
			cons.pbDeserializer = newDeserializer()
		default:
			cons.schemaType = srclient.Avro
		}

		cons.schemaRegistryOn = true
	}

	if cons.groupMode {
		cg, err := sarama.NewConsumerGroup(cc.Brokers, cc.GroupID, sc)
		if err != nil {
			return nil, err
		}
		cons.cg = cg
		cons.fetchCh = make(chan *sarama.ConsumerMessage)
		cons.commitCh = make(chan *sarama.ConsumerMessage)
		cons.consumeErrCh = make(chan error)

		ctx := context.Background()
		ctx, cons.cancel = context.WithCancel(ctx)

		go func(topic string) {
			topics := []string{topic}
			for {
				if err := cons.cg.Consume(ctx, topics, cons); err != nil {
					cons.consumeErrCh <- err
				}
			}
		}(cc.Topic)
	} else {
		c, err := sarama.NewConsumer(cc.Brokers, sc)
		if err != nil {
			return nil, err
		}
		cons.c = c

		pc, err := cons.c.ConsumePartition(cc.Topic, int32(cc.Partition), sarama.OffsetOldest)
		if err != nil {
			return nil, err
		}
		cons.pc = pc
	}

	return cons, nil
}

// NetInfo returns information about whether SASL and TLS are enabled.
func (c *saramaConsumer) NetInfo() string {
	saslInfo := "SASL disabled"
	if c.saslOn {
		saslInfo = "SASL enabled"
	}

	tlsInfo := "TLS disabled"
	if c.tlsOn {
		tlsInfo = "TLS enabled"
	}
	if c.tlsSkipVerify {
		tlsInfo += " (insecure skip verify)"
	}

	return fmt.Sprintf("%s, %s", saslInfo, tlsInfo)
}

// Fetch reads an incoming message. In group mode, the message is outstanding
// until committed.
func (c *saramaConsumer) Fetch(ctx context.Context) (Message, error) {

	if c.groupMode {
		select {
		case <-ctx.Done():
			return Message{}, ctx.Err()
		case cmsg := <-c.fetchCh:
			var deserializedValue = cmsg.Value
			var err error
			if c.schemaRegistryOn {
				deserializedValue, err = c.deserializePayload(cmsg.Value)
			}

			if err == nil {
				return Message{
					Topic:     cmsg.Topic,
					Partition: int(cmsg.Partition),
					Offset:    cmsg.Offset,

					Key:     cmsg.Key,
					Value:   deserializedValue,
					Headers: c.convertToMessageHeaders(cmsg.Headers),
				}, nil
			}
			return Message{}, err
		case loopErr := <-c.consumeErrCh:
			return Message{}, loopErr
		}
	}

	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case cmsg := <-c.pc.Messages():
		var deserializedValue = cmsg.Value
		var err error
		if c.schemaRegistryOn {
			deserializedValue, err = c.deserializePayload(cmsg.Value)
		}

		if err == nil {
			return Message{
				Topic:     cmsg.Topic,
				Partition: int(cmsg.Partition),
				Offset:    cmsg.Offset,

				Key:     cmsg.Key,
				Value:   deserializedValue,
				Headers: c.convertToMessageHeaders(cmsg.Headers),
			}, nil
		}
		return Message{}, err
	}
}

// Commit commits a message. This is only available in group mode.
func (c *saramaConsumer) Commit(ctx context.Context, m Message) error {
	if !c.groupMode {
		return fmt.Errorf("commit is only available in group mode")
	}

	cmsg := &sarama.ConsumerMessage{
		Topic:     m.Topic,
		Partition: int32(m.Partition),
		Offset:    m.Offset,

		Key:   m.Key,
		Value: m.Value,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.commitCh <- cmsg:
	case loopErr := <-c.consumeErrCh:
		return loopErr
	}
	return nil
}

// GroupMode returns whether the consumer is in group mode or not.
func (c *saramaConsumer) GroupMode() bool {
	return c.groupMode
}

// Close closes the underlying Kafka consumer connection.
func (c *saramaConsumer) Close() error {
	if c.groupMode {
		close(c.fetchCh)
		close(c.commitCh)
		c.cancel()
		return c.cg.Close()
	}

	if err := c.pc.Close(); err != nil {
		c.c.Close()
		return err
	}
	return c.c.Close()
}

// Setup is a no-op. It only exists to satisfy sarama.ConsumerGroupHandler.
func (c *saramaConsumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is a no-op. It only exists to satisfy sarama.ConsumerGroupHandler.
func (c *saramaConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes incoming consumer group messages. This satisfies
// sarama.ConsumerGroupHandler.
func (c *saramaConsumer) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for cmsg := range claim.Messages() {
		c.fetchCh <- cmsg

		cmsg = <-c.commitCh
		sess.MarkMessage(cmsg, "")
	}

	return nil
}

// Retrieve the schema of the message and deserialize it.
func (c *saramaConsumer) deserializePayload(payload []byte) ([]byte, error) {
	// first byte of the payload is 0
	if payload[0] != byte(0) {
		return nil, fmt.Errorf("failed to deserialize payload: magic byte is not 0")
	}

	// next 4 bytes contain the schema id
	schemaID := binary.BigEndian.Uint32(payload[1:5])
	schema, err := c.schemaRegistryClient.GetSchema(int(schemaID))
	if err != nil {
		return nil, err
	}

	var value []byte
	switch c.schemaType {
	case srclient.Avro:
		value, err = c.deserializeAvro(schema, payload[5:])
	case srclient.Json:
		value, err = c.validateJSONSchema(schema, payload[5:])
	case srclient.Protobuf:
		value, err = c.pbDeserializer.Deserialize(schema, payload[5:])
	}

	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *saramaConsumer) deserializeAvro(schema *srclient.Schema, cleanPayload []byte) ([]byte, error) {
	codec := schema.Codec()
	native, _, err := codec.NativeFromBinary(cleanPayload)
	if err != nil {
		return nil, fmt.Errorf("unable to deserailize avro: %w", err)
	}
	value, err := codec.TextualFromNative(nil, native)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to json: %w", err)
	}

	return value, nil
}

func (c *saramaConsumer) validateJSONSchema(schema *srclient.Schema, cleanPayload []byte) ([]byte, error) {
	jsc, err := jsonschema.CompileString("schema.json", schema.Schema())
	if err != nil {
		return nil, fmt.Errorf("unable to parse json schema: %w", err)
	}

	var parsedMessage interface{}
	err = json.Unmarshal(cleanPayload, &parsedMessage)
	if err != nil {
		return nil, fmt.Errorf("unable to parse json message: %w", err)
	}

	err = jsc.Validate(parsedMessage)
	if err != nil {
		return nil, fmt.Errorf("json message validation failed: %w", err)
	}

	return cleanPayload, nil
}

func (c *saramaConsumer) convertToMessageHeaders(consumerHeaders []*sarama.RecordHeader) []sarama.RecordHeader {
	var msgHeaders = make([]sarama.RecordHeader, len(consumerHeaders))
	for i, element := range consumerHeaders {
		msgHeaders[i] = *element
	}
	return msgHeaders
}
