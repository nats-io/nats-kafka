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
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/kafka"
)

// Kafka2StanConnector connects Kafka topic to a nats streaming channel
type Kafka2StanConnector struct {
	BridgeConnector

	reader     kafka.Consumer
	shutdownCB ShutdownCallback
}

// NewKafka2StanConnector create a new Kafka to STAN connector
func NewKafka2StanConnector(bridge *NATSKafkaBridge, config conf.ConnectorConfig) Connector {
	connector := &Kafka2StanConnector{}
	connector.init(bridge, config, config.Channel, fmt.Sprintf("Kafka:%s to Stan:%s", config.Topic, config.Channel))
	return connector
}

// Start the connector
func (conn *Kafka2StanConnector) Start() error {
	conn.Lock()
	defer conn.Unlock()

	if !conn.bridge.CheckStan() {
		return fmt.Errorf("%s connector requires nats streaming to be available", conn.String())
	}

	conn.bridge.Logger().Tracef("starting connection %s", conn.String())

	var err error
	dialTimeout := time.Duration(conn.bridge.config.ConnectTimeout) * time.Millisecond
	conn.reader, err = kafka.NewConsumer(conn.config, dialTimeout)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	if s, ok := conn.reader.(interface{ NetInfo() string }); ok {
		conn.bridge.Logger().Noticef(s.NetInfo())
	}

	cb, err := conn.setUpListener(conn.reader, conn.stanMessageHandler)
	if err != nil {
		return err
	}
	conn.shutdownCB = cb

	conn.stats.AddConnect()
	conn.bridge.Logger().Tracef("opened and reading %s", conn.config.Topic)
	conn.bridge.Logger().Noticef("started connection %s", conn.String())

	return nil
}

// Shutdown the connector
func (conn *Kafka2StanConnector) Shutdown() error {
	conn.Lock()
	defer conn.Unlock()
	conn.closeWriters()
	conn.stats.AddDisconnect()

	conn.bridge.Logger().Noticef("shutting down connection %s", conn.String())

	if conn.shutdownCB != nil {
		if err := conn.shutdownCB(); err != nil {
			conn.bridge.Logger().Noticef("error stopping listen routine for %s, %s", conn.String(), err.Error())
		}
		conn.shutdownCB = nil
	}

	reader := conn.reader
	conn.reader = nil

	if reader != nil {
		if err := reader.Close(); err != nil {
			conn.bridge.Logger().Noticef("error closing reader for %s, %s", conn.String(), err.Error())
		}
	}

	return nil // ignore the disconnect error
}

// CheckConnections ensures the nats/stan connection and report an error if it is down
func (conn *Kafka2StanConnector) CheckConnections() error {
	if !conn.bridge.CheckStan() {
		return fmt.Errorf("%s connector requires nats streaming to be available", conn.String())
	}
	return nil
}
