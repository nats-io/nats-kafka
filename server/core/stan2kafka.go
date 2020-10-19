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
 *
 */

package core

import (
	"fmt"

	"github.com/nats-io/stan.go"

	"github.com/nats-io/nats-kafka/server/conf"
)

// Stan2KafkaConnector connects a STAN channel to Kafka
type Stan2KafkaConnector struct {
	BridgeConnector
	sub stan.Subscription
}

// NewStan2KafkaConnector create a new stan to kafka
func NewStan2KafkaConnector(bridge *NATSKafkaBridge, config conf.ConnectorConfig) Connector {
	connector := &Stan2KafkaConnector{}
	connector.init(bridge, config, config.Topic, fmt.Sprintf("Stan:%s to Kafka:%s", config.Channel, config.Topic))
	return connector
}

// Start the connector
func (conn *Stan2KafkaConnector) Start() error {
	conn.Lock()
	defer conn.Unlock()

	if !conn.bridge.CheckStan() {
		return fmt.Errorf("%s connector requires nats streaming to be available", conn.String())
	}

	conn.bridge.Logger().Tracef("starting connection %s", conn.String())

	sub, err := conn.subscribeToChannel()
	if err != nil {
		return err
	}
	conn.sub = sub

	conn.stats.AddConnect()
	conn.bridge.Logger().Tracef("opened and reading %s", conn.config.Channel)
	conn.bridge.Logger().Noticef("started connection %s", conn.String())

	return nil
}

// Shutdown the connector
func (conn *Stan2KafkaConnector) Shutdown() error {
	conn.Lock()
	defer conn.Unlock()
	conn.closeWriters()
	conn.stats.AddDisconnect()

	conn.bridge.Logger().Noticef("shutting down connection %s", conn.String())

	if conn.sub != nil && conn.config.DurableName == "" { // Don't unsubscribe from durables
		conn.bridge.Logger().Tracef("unsubscribing from %s", conn.config.Channel)
		conn.sub.Unsubscribe()
		conn.sub = nil
	}

	return nil
}

// CheckConnections ensures the nats/stan connection and report an error if it is down
func (conn *Stan2KafkaConnector) CheckConnections() error {
	if !conn.bridge.CheckStan() {
		return fmt.Errorf("%s connector requires nats streaming to be available", conn.String())
	}
	return nil
}
