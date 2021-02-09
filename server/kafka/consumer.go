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
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
)

type Message struct {
	Topic     string
	Partition int
	Offset    int64

	Key   []byte
	Value []byte
}

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
}

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

		fetchCh:      make(chan *sarama.ConsumerMessage),
		commitCh:     make(chan *sarama.ConsumerMessage),
		consumeErrCh: make(chan error),
	}

	if cons.groupMode {
		cg, err := sarama.NewConsumerGroup(cc.Brokers, cc.GroupID, sc)
		if err != nil {
			return nil, err
		}
		cons.cg = cg

		go func(topic string) {
			ctx := context.Background()
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

func (c *saramaConsumer) Fetch(ctx context.Context) (Message, error) {
	if c.groupMode {
		select {
		case <-ctx.Done():
			return Message{}, ctx.Err()
		case cmsg := <-c.fetchCh:
			return Message{
				Topic:     cmsg.Topic,
				Partition: int(cmsg.Partition),
				Offset:    cmsg.Offset,

				Key:   cmsg.Key,
				Value: cmsg.Value,
			}, nil
		case loopErr := <-c.consumeErrCh:
			return Message{}, loopErr
		}
	}

	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case cmsg := <-c.pc.Messages():
		return Message{
			Topic:     cmsg.Topic,
			Partition: int(cmsg.Partition),
			Offset:    cmsg.Offset,

			Key:   cmsg.Key,
			Value: cmsg.Value,
		}, nil
	}
}

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

func (c *saramaConsumer) GroupMode() bool {
	return c.groupMode
}

func (c *saramaConsumer) Close() error {
	if c.groupMode {
		close(c.fetchCh)
		close(c.commitCh)
		return c.cg.Close()
	}

	if err := c.pc.Close(); err != nil {
		c.c.Close()
		return err
	}
	return c.c.Close()
}

func (c *saramaConsumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (c *saramaConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (c *saramaConsumer) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for cmsg := range claim.Messages() {
		c.fetchCh <- cmsg

		cmsg = <-c.commitCh
		sess.MarkMessage(cmsg, "")
	}

	return nil
}
