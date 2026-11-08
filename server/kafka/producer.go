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
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/riferrei/srclient"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
)

// Producer represents a Kafka producer.
type Producer interface {
	Write(Message) error
	Close() error
}

type saramaProducer struct {
	sp    sarama.SyncProducer
	topic string

	saslOn        bool
	tlsOn         bool
	tlsSkipVerify bool

	schemaRegistryOn     bool
	schemaRegistryClient srclient.ISchemaRegistryClient
	subjectName          string
	schemaVersion        int
	schemaType           srclient.SchemaType
	pbSerializer         pbSerializer
}

// IsTopicExist returns whether an error is caused by a topic already existing.
func IsTopicExist(err error) bool {
	var terr *sarama.TopicError
	if !errors.As(err, &terr) {
		return false
	}
	return terr.Err == sarama.ErrTopicAlreadyExists
}

// NewProducer returns a new Kafka Producer.
func NewProducer(cc conf.ConnectorConfig, bc conf.NATSKafkaBridgeConfig, topic string) (Producer, error) {
	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	sc.Net.DialTimeout = time.Duration(bc.ConnectTimeout) * time.Millisecond
	sc.ClientID = "nats-kafka-producer"

	if cc.Balancer == conf.LeastBytes {
		sc.Producer.Partitioner = NewLeastBytesPartitioner
	}

	if cc.SASL.User != "" {
		sc.Net.SASL.Enable = true
		sc.Net.SASL.Handshake = true
		if cc.SASL.Mechanism == "scram-sha256" || cc.SASL.Mechanism == "scram-sha-256" || cc.SASL.Mechanism == "sha256" {
			sc.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
			sc.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		} else if cc.SASL.Mechanism == "scram-sha512" || cc.SASL.Mechanism == "scram-sha-512" || cc.SASL.Mechanism == "sha512" {
			sc.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
			sc.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		} else {
			sc.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
		sc.Net.SASL.User = cc.SASL.User
		sc.Net.SASL.Password = cc.SASL.Password
	} else if cc.IAM.Enable && cc.IAM.Region != "" {
		sc.Net.SASL.Enable = true
		sc.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		sc.Net.SASL.TokenProvider = &MSKAccessTokenProvider{Region: cc.IAM.Region}
	}

	if sc.Net.SASL.Enable && cc.SASL.InsecureSkipVerify {
		sc.Net.TLS.Enable = true
		sc.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: cc.SASL.InsecureSkipVerify,
		}
	} else if tlsC, err := cc.TLS.MakeTLSConfig(); tlsC != nil && err == nil {
		sc.Net.TLS.Enable = true
		sc.Net.TLS.Config = tlsC
	} else if cc.IAM.Enable {
		tlsConfig := tls.Config{}
		sc.Net.TLS.Enable = true
		sc.Net.TLS.Config = &tlsConfig
	}
	if cc.SASL.TLS {
		sc.Net.TLS.Enable = cc.SASL.TLS
	}

	sp, err := sarama.NewSyncProducer(cc.Brokers, sc)
	if err != nil {
		return nil, err
	}

	prod := &saramaProducer{
		sp:            sp,
		topic:         topic,
		saslOn:        sc.Net.SASL.Enable,
		tlsOn:         sc.Net.TLS.Enable,
		tlsSkipVerify: cc.SASL.InsecureSkipVerify,
	}

	// If schema registry url and subject name both are set, enable schema registry integration
	if cc.SchemaRegistryURL != "" && cc.SubjectName != "" {
		prod.schemaRegistryClient = srclient.CreateSchemaRegistryClient(cc.SchemaRegistryURL)
		prod.subjectName = cc.SubjectName
		prod.schemaVersion = cc.SchemaVersion

		switch strings.ToUpper(cc.SchemaType) {
		case srclient.Json.String():
			prod.schemaType = srclient.Json
		case srclient.Protobuf.String():
			prod.schemaType = srclient.Protobuf
			prod.pbSerializer = newSerializer()
		default:
			prod.schemaType = srclient.Avro
		}

		prod.schemaRegistryOn = true
	}

	return prod, nil
}

// NetInfo returns information about whether SASL and TLS are enabled.
func (p *saramaProducer) NetInfo() string {
	saslInfo := "SASL disabled"
	if p.saslOn {
		saslInfo = "SASL enabled"
	}

	tlsInfo := "TLS disabled"
	if p.tlsOn {
		tlsInfo = "TLS enabled"
	}
	if p.tlsSkipVerify {
		tlsInfo += " (insecure skip verify)"
	}

	return fmt.Sprintf("%s, %s", saslInfo, tlsInfo)
}

// subjectHeaderKey is the Kafka header (populated for JetStream sources) that
// carries the concrete NATS subject of the delivered message.
const subjectHeaderKey = "subject"

// Write sends an outgoing message.
func (p *saramaProducer) Write(m Message) error {
	var valueEncoder sarama.Encoder
	if p.schemaRegistryOn {
		encodedValue, err := p.serializePayload(m.Value)
		if err != nil {
			return err
		}
		valueEncoder = sarama.ByteEncoder(encodedValue)
	} else {
		valueEncoder = sarama.StringEncoder(m.Value)
	}

	key := refineKeyWithSubjectHeader(m.Key, m.Headers)

	_, _, err := p.sp.SendMessage(&sarama.ProducerMessage{
		Topic:   p.topic,
		Value:   valueEncoder,
		Key:     sarama.StringEncoder(key),
		Headers: m.Headers,
	})
	return err
}

// refineKeyWithSubjectHeader refines a wildcard-terminated key such as
// "STATE.GLOBAL_V2C.19.CELL2.>" into "STATE.GLOBAL_V2C.19.CELL2.01-244530648.>"
// by taking the first 5 dot-separated tokens of the message's "subject" header
// (through the car_id level) and re-appending the ">" wildcard.
//
// The original key is returned unchanged when the key does not end in ">",
// there is no "subject" header, or the subject has fewer than 5 tokens.
func refineKeyWithSubjectHeader(key []byte, headers []sarama.RecordHeader) []byte {
	if len(key) == 0 || key[len(key)-1] != '>' {
		return key
	}

	subject, found := "", false
	for _, h := range headers {
		if string(h.Key) == subjectHeaderKey {
			subject, found = string(h.Value), true
			break
		}
	}
	if !found {
		return key
	}

	subjTokens := strings.Split(subject, ".")
	if len(subjTokens) < 5 {
		return key // subject is not specific enough to build the key
	}

	return []byte(strings.Join(subjTokens[:5], ".") + ".>")
}

// Close closes the underlying Kafka connection. It blocks until all messages
// are sent.
func (p *saramaProducer) Close() error {
	return p.sp.Close()
}

type erroredProducer struct {
	err error
}

// NewErroredProducer returns a Producer that fails when any methods are
// called.
func NewErroredProducer(err error) Producer {
	return &erroredProducer{err: err}
}

func (p *erroredProducer) Write(m Message) error {
	return p.err
}

func (p *erroredProducer) Close() error {
	return p.err
}

// Retrieve the schema from the schema registry and serialize the message. This method expects data in Avro JSON format
// for cross language compatibility.
func (p *saramaProducer) serializePayload(jsonPayload []byte) ([]byte, error) {
	var schema *srclient.Schema
	var err error
	if p.schemaRegistryOn && p.schemaVersion != 0 {
		schema, err = p.schemaRegistryClient.GetSchemaByVersion(p.subjectName, p.schemaVersion)
	} else {
		// Version is not set, fetch and use the latest
		schema, err = p.schemaRegistryClient.GetLatestSchema(p.subjectName)
	}

	if err != nil {
		return nil, err
	}

	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(schema.ID()))

	var valueBytes []byte
	switch p.schemaType {
	case srclient.Avro:
		valueBytes, err = p.serializeAvro(schema, jsonPayload)
	case srclient.Json:
		valueBytes = jsonPayload
	case srclient.Protobuf:
		valueBytes, err = p.pbSerializer.Serialize(schema, jsonPayload)
	}

	if err != nil {
		return nil, err
	}

	var recordValue []byte
	recordValue = append(recordValue, byte(0))
	recordValue = append(recordValue, schemaIDBytes...)
	recordValue = append(recordValue, valueBytes...)

	return recordValue, nil
}

func (p *saramaProducer) serializeAvro(schema *srclient.Schema, payload []byte) ([]byte, error) {
	codec := schema.Codec()
	native, _, err := codec.NativeFromTextual(payload)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize json: %w", err)
	}
	value, err := codec.BinaryFromNative(nil, native)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to avro: %w", err)
	}

	return value, err
}
