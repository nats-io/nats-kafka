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
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
)

type Manager interface {
	CreateTopic(topic string, partitions, replication int) error
	Close() error
}

type saramaManager struct {
	ca sarama.ClusterAdmin
}

func NewManager(cc conf.ConnectorConfig, dialTimeout time.Duration) (Manager, error) {
	sc := sarama.NewConfig()
	sc.Net.DialTimeout = dialTimeout
	sc.ClientID = "nats-kafka-manager"

	if cc.SASL.User != "" {
		sc.Net.SASL.Enable = true
		sc.Net.SASL.User = cc.SASL.User
		sc.Net.SASL.Password = cc.SASL.Password
	} else if tlsC, err := cc.TLS.MakeTLSConfig(); err == nil {
		sc.Net.TLS.Enable = (tlsC != nil)
		sc.Net.TLS.Config = tlsC
	}

	ca, err := sarama.NewClusterAdmin(cc.Brokers, sc)
	if err != nil {
		return nil, err
	}

	return &saramaManager{ca: ca}, nil
}

func (m *saramaManager) CreateTopic(topic string, partitions, replication int) error {
	return m.ca.CreateTopic(topic, &sarama.TopicDetail{
		NumPartitions:     int32(partitions),
		ReplicationFactor: int16(replication),
	}, false)
}

func (m *saramaManager) Close() error {
	return m.ca.Close()
}
