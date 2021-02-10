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
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
)

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
}

func IsTopicExist(err error) bool {
	var terr *sarama.TopicError
	if !errors.As(err, &terr) {
		return false
	}
	return terr.Err == sarama.ErrTopicAlreadyExists
}

func NewProducer(cc conf.ConnectorConfig, bc conf.NATSKafkaBridgeConfig, topic string) (Producer, error) {
	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	sc.Net.DialTimeout = time.Duration(bc.ConnectTimeout) * time.Millisecond
	sc.ClientID = "nats-kafka-producer"

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

	sp, err := sarama.NewSyncProducer(cc.Brokers, sc)
	if err != nil {
		return nil, err
	}

	return &saramaProducer{
		sp:            sp,
		topic:         topic,
		saslOn:        sc.Net.SASL.Enable,
		tlsOn:         sc.Net.TLS.Enable,
		tlsSkipVerify: cc.SASL.InsecureSkipVerify,
	}, nil
}

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

func (p *saramaProducer) Write(m Message) error {
	_, _, err := p.sp.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.StringEncoder(m.Value),
		Key:   sarama.StringEncoder(m.Key),
	})
	return err
}

func (p *saramaProducer) Close() error {
	return p.sp.Close()
}

type erroredProducer struct {
	err error
}

func NewErroredProducer(err error) Producer {
	return &erroredProducer{err: err}
}

func (p *erroredProducer) Write(m Message) error {
	return p.err
}

func (p *erroredProducer) Close() error {
	return p.err
}
