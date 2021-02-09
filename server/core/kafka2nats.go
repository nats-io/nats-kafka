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

// Kafka2NATSConnector connects Kafka topic to a NATS subject
type Kafka2NATSConnector struct {
	BridgeConnector

	reader     kafka.Consumer
	shutdownCB ShutdownCallback
}

// NewKafka2NATSConnector create a new Kafka to NATS connector
func NewKafka2NATSConnector(bridge *NATSKafkaBridge, config conf.ConnectorConfig) Connector {
	connector := &Kafka2NATSConnector{}
	connector.init(bridge, config, config.Subject, fmt.Sprintf("Kafka:%s to NATS:%s", config.Topic, config.Subject))
	return connector
}

// Start the connector
func (conn *Kafka2NATSConnector) Start() error {
	conn.Lock()
	defer conn.Unlock()

	if !conn.bridge.CheckNATS() {
		return fmt.Errorf("%s connector requires nats to be available", conn.String())
	}

	conn.bridge.Logger().Tracef("starting connection %s", conn.String())

	dialTimeout := time.Duration(conn.bridge.config.ConnectTimeout) * time.Millisecond
	r, err := kafka.NewConsumer(conn.config, dialTimeout)
	if err != nil {
		return err
	}
	conn.reader = r
	if s, ok := conn.reader.(interface{ NetInfo() string }); ok {
		conn.bridge.Logger().Noticef(s.NetInfo())
	}

	cb, err := conn.setUpListener(conn.reader, conn.natsMessageHandler)
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
func (conn *Kafka2NATSConnector) Shutdown() error {
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
func (conn *Kafka2NATSConnector) CheckConnections() error {
	if !conn.bridge.CheckNATS() {
		return fmt.Errorf("%s connector requires nats to be available", conn.String())
	}
	return nil
}
