/*
 * Copyright 2019 The NATS Authors
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
 */

package core

import (
	"fmt"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats.go"
)

// NATS2KafkaConnector connects a NATS subject to a Kafka topic
type NATS2KafkaConnector struct {
	BridgeConnector
	sub *nats.Subscription
}

// NewNATS2KafkaConnector create a nats to MQ connector
func NewNATS2KafkaConnector(bridge *NATSKafkaBridge, config conf.ConnectorConfig) Connector {
	connector := &NATS2KafkaConnector{}
	connector.init(bridge, config, config.Topic, fmt.Sprintf("NATS:%s to Kafka:%s", config.Subject, config.Topic))
	return connector
}

// Start the connector
func (conn *NATS2KafkaConnector) Start() error {
	conn.Lock()
	defer conn.Unlock()

	if !conn.bridge.CheckNATS() {
		return fmt.Errorf("%s connector requires nats to be available", conn.String())
	}

	conn.bridge.Logger().Tracef("starting connection %s", conn.String())

	sub, err := conn.subscribeToNATS(conn.config.Subject, conn.config.QueueName)
	if err != nil {
		return err
	}
	conn.sub = sub

	conn.stats.AddConnect()
	conn.bridge.Logger().Tracef("opened and reading %s", conn.config.Topic)
	conn.bridge.Logger().Noticef("started connection %s", conn.String())

	return nil
}

// Shutdown the connector
func (conn *NATS2KafkaConnector) Shutdown() error {
	conn.Lock()
	defer conn.Unlock()
	conn.closeWriters()
	conn.stats.AddDisconnect()

	conn.bridge.Logger().Noticef("shutting down connection %s", conn.String())

	if conn.sub != nil {
		conn.sub.Unsubscribe()
		conn.sub = nil
	}

	return nil
}

// CheckConnections ensures the nats/stan connection and report an error if it is down
func (conn *NATS2KafkaConnector) CheckConnections() error {
	if !conn.bridge.CheckNATS() {
		return fmt.Errorf("%s connector requires nats to be available", conn.String())
	}
	return nil
}
