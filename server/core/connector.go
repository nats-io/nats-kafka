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
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nuid"
	"github.com/segmentio/kafka-go"
)

// Connector is the abstraction for all of the bridge connector types
type Connector interface {
	Start() error
	Shutdown() error

	CheckConnections() error

	String() string
	ID() string

	Stats() ConnectorStats
}

// CreateConnector builds a connector from the supplied configuration
func CreateConnector(config conf.ConnectorConfig, bridge *NATSKafkaBridge) (Connector, error) {
	switch config.Type {
	case conf.NATSToKafka:
		return NewNATS2KafkaConnector(bridge, config), nil
	case conf.STANToKafka:
		return NewStan2KafkaConnector(bridge, config), nil
	case conf.KafkaToNATS:
		return NewKafka2NATSConnector(bridge, config), nil
	case conf.KafkaToStan:
		return NewKafka2StanConnector(bridge, config), nil
	default:
		return nil, fmt.Errorf("unknown connector type %q in configuration", config.Type)
	}
}

// BridgeConnector is the base type used for connectors so that they can share code
// The config, bridge and stats are all fixed at creation, so no lock is required on the
// connector at this level. The stats do keep a lock to protect their data.
// The connector has a lock for use by composing types to protect themselves during start/shutdown.
type BridgeConnector struct {
	sync.Mutex

	config conf.ConnectorConfig
	bridge *NATSKafkaBridge
	stats  *ConnectorStatsHolder
}

// Start is a no-op, designed for overriding
func (conn *BridgeConnector) Start() error {
	return nil
}

// Shutdown is a no-op, designed for overriding
func (conn *BridgeConnector) Shutdown() error {
	return nil
}

// CheckConnections is a no-op, designed for overriding
// This is called when nats or stan goes down
// the connector should return an error if it has to be shut down
func (conn *BridgeConnector) CheckConnections() error {
	return nil
}

// String returns the name passed into init
func (conn *BridgeConnector) String() string {
	return conn.stats.Name()
}

// ID returns the id from the stats
func (conn *BridgeConnector) ID() string {
	return conn.stats.ID()
}

// Stats returns a copy of the current stats for this connector
func (conn *BridgeConnector) Stats() ConnectorStats {
	return conn.stats.Stats()
}

// Init sets up common fields for all connectors
func (conn *BridgeConnector) init(bridge *NATSKafkaBridge, config conf.ConnectorConfig, name string) {
	conn.config = config
	conn.bridge = bridge

	id := conn.config.ID
	if id == "" {
		id = nuid.Next()
	}
	conn.stats = NewConnectorStatsHolder(name, id)
}

// NATSCallback used by conn-nats connectors in an conn library callback
// The lock will be held by the caller!
type NATSCallback func(natsMsg []byte) error

// ShutdownCallback is returned when setting up a callback or polling so the connector can shut it down
type ShutdownCallback func() error

func (conn *BridgeConnector) stanMessageHandler(natsMsg []byte) error {
	return conn.bridge.Stan().Publish(conn.config.Channel, natsMsg)
}

func (conn *BridgeConnector) natsMessageHandler(natsMsg []byte) error {
	return conn.bridge.NATS().Publish(conn.config.Subject, natsMsg)
}

func (conn *BridgeConnector) calculateKey(subject string, replyto string) []byte {
	keyType := conn.config.KeyType
	keyValue := conn.config.KeyValue

	if keyType == conf.FixedKey {
		return []byte(keyValue)
	}

	if keyType == conf.SubjectKey {
		return []byte(subject)
	}

	if keyType == conf.ReplyToKey {
		return []byte(replyto)
	}

	if keyType == conf.SubjectRegex {
		r, err := regexp.Compile(keyValue)

		if err != nil {
			conn.bridge.logger.Noticef("invalid regex for %s key value", conn.String())
			return []byte{}
		}

		result := r.FindStringSubmatch(subject)

		if len(result) > 1 {
			return []byte(result[1])
		}

		return []byte{}
	}

	if keyType == conf.ReplyRegex {
		r, err := regexp.Compile(keyValue)

		if err != nil {
			conn.bridge.logger.Noticef("invalid regex for %s key value", conn.String())
			return []byte{}
		}

		result := r.FindStringSubmatch(replyto)

		if len(result) > 1 {
			return []byte(result[1])
		}

		return []byte{}
	}

	return []byte{} // empty key by default
}

// set up a nats subscription, assumes the lock is held
func (conn *BridgeConnector) subscribeToNATS(subject string, natsQueue string, dest *kafka.Writer) (*nats.Subscription, error) {
	traceEnabled := conn.bridge.Logger().TraceEnabled()
	callback := func(msg *nats.Msg) {
		start := time.Now()
		l := int64(len(msg.Data))

		// send to kafka here
		err := dest.WriteMessages(context.Background(),
			kafka.Message{
				Key:   conn.calculateKey(msg.Subject, msg.Reply),
				Value: msg.Data,
			})

		if err != nil {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s wrote message to kafka", conn.String())
			}
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Noticef("connector publish failure, %s, %s", conn.String(), err.Error())
		} else {
			conn.stats.AddRequest(l, l, time.Since(start))
		}
	}

	if conn.bridge.NATS() == nil {
		return nil, fmt.Errorf("bridge not configured to use NATS streaming")
	}

	if natsQueue == "" {
		return conn.bridge.NATS().Subscribe(subject, callback)
	}

	return conn.bridge.NATS().QueueSubscribe(subject, natsQueue, callback)
}

// subscribeToChannel uses the bridges STAN connection to subscribe based on the config
// The start position/time and durable name are optional
func (conn *BridgeConnector) subscribeToChannel(dest *kafka.Writer) (stan.Subscription, error) {
	if conn.bridge.Stan() == nil {
		return nil, fmt.Errorf("bridge not configured to use NATS streaming")
	}

	options := []stan.SubscriptionOption{}

	if conn.config.DurableName != "" {
		options = append(options, stan.DurableName(conn.config.DurableName))
	}

	if conn.config.StartAtTime != 0 {
		t := time.Unix(conn.config.StartAtTime, 0)
		options = append(options, stan.StartAtTime(t))
	} else if conn.config.StartAtSequence == -1 {
		options = append(options, stan.StartWithLastReceived())
	} else if conn.config.StartAtSequence > 0 {
		options = append(options, stan.StartAtSequence(uint64(conn.config.StartAtSequence)))
	} else {
		options = append(options, stan.DeliverAllAvailable())
	}

	options = append(options, stan.SetManualAckMode())
	traceEnabled := conn.bridge.Logger().TraceEnabled()

	callback := func(msg *stan.Msg) {
		start := time.Now()
		l := int64(len(msg.Data))

		if traceEnabled {
			conn.bridge.Logger().Tracef("%s received message", conn.String())
		}

		key := conn.calculateKey(conn.config.Channel, conn.config.DurableName)
		err := dest.WriteMessages(context.Background(),
			kafka.Message{
				Key:   key,
				Value: msg.Data,
			})

		if err != nil {
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Noticef("connector publish failure, %s, %s", conn.String(), err.Error())
		} else {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s wrote message to kafka with key %s", conn.String(), string(key))
			}
			msg.Ack()
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s acked message to kafka", conn.String())
			}
			conn.stats.AddRequest(l, l, time.Since(start))
		}
	}

	sub, err := conn.bridge.Stan().Subscribe(conn.config.Channel, callback, options...)

	return sub, err
}

func (conn *BridgeConnector) connectWriter() *kafka.Writer {
	config := conn.config

	dialer := &kafka.Dialer{
		Timeout:   time.Duration(conn.bridge.config.ConnectTimeout) * time.Millisecond,
		DualStack: true,
	}

	tlsC, err := config.TLS.MakeTLSConfig()

	if err != nil {
		conn.bridge.Logger().Noticef("TLS config error for %s, %s", conn.String(), err.Error())
		return nil
	}

	if tlsC == nil {
		conn.bridge.Logger().Noticef("TLS disabled for %s", conn.String())
	} else {
		dialer.TLS = tlsC
	}

	var balancer kafka.Balancer

	if config.Balancer == conf.LeastBytes {
		balancer = &kafka.LeastBytes{}
	} else { // default to hash
		balancer = &kafka.Hash{}
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:   conn.config.Brokers,
		Topic:     conn.config.Topic,
		Balancer:  balancer,
		Dialer:    dialer,
		BatchSize: 1,
	})

	return writer
}

func (conn *BridgeConnector) connectReader() *kafka.Reader {
	config := conn.config

	dialer := &kafka.Dialer{
		Timeout:   time.Duration(conn.bridge.config.ConnectTimeout) * time.Millisecond,
		DualStack: true,
	}

	tlsC, err := config.TLS.MakeTLSConfig()

	if err != nil {
		conn.bridge.Logger().Noticef("TLS config error for %s, %s", conn.String(), err.Error())
		return nil
	}

	if tlsC == nil {
		conn.bridge.Logger().Noticef("TLS disabled for %s", conn.String())
	} else {
		dialer.TLS = tlsC
	}

	readerConfig := kafka.ReaderConfig{
		Brokers:  conn.config.Brokers,
		Topic:    config.Topic,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
		Dialer:   dialer,
	}

	if config.MinBytes > 0 {
		readerConfig.MinBytes = int(config.MinBytes)
	}

	if config.MaxBytes > 0 {
		readerConfig.MaxBytes = int(config.MaxBytes)
	}

	if config.GroupID != "" {
		readerConfig.GroupID = config.GroupID
	} else if config.Partition >= 0 {
		readerConfig.Partition = int(config.Partition)
	}

	return kafka.NewReader(readerConfig)
}

func (conn *BridgeConnector) setUpListener(target *kafka.Reader, natsCallbackFunc NATSCallback) (ShutdownCallback, error) {
	done := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(1)

	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	traceEnabled := conn.bridge.Logger().TraceEnabled()

	listenerCallbackFunc := func(conn *BridgeConnector, msg kafka.Message) {
		start := time.Now()
		value := msg.Value
		l := int64(len(value))
		err := natsCallbackFunc(value)

		if err != nil {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s received message from kafka", conn.String())
			}
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Noticef("publish failure for %s, %s", conn.String(), err.Error())
			return
		}

		if conn.config.GroupID != "" {
			err := target.CommitMessages(ctx, msg)

			if err != nil {
				conn.stats.AddMessageIn(l)
				conn.bridge.Logger().Noticef("failed to commit, %s", err.Error())
				go conn.bridge.ConnectorError(conn, err) // run in a go routine so we can finish this method
				return
			}

			if traceEnabled {
				conn.bridge.Logger().Tracef("%s committed message from kafka", conn.String())
			}
		}

		conn.stats.AddRequest(l, l, time.Since(start))
	}

	conn.bridge.Logger().Tracef("starting listener for %s", conn.String())

	go func() {
		for {
			msg, err := target.FetchMessage(cancelCtx)

			if err != nil {
				if err == cancelCtx.Err() {
					wg.Done()
					return
				}

				conn.bridge.Logger().Noticef("error fetching message, %s", err.Error())
				go conn.bridge.ConnectorError(conn, err) // run in a go routine so we can finish this method and unlock
				wg.Done()
				return
			}

			listenerCallbackFunc(conn, msg)

			select {
			case <-done:
				wg.Done()
				return
			default:
			}
		}
	}()

	return func() error {
		close(done)
		cancelFunc()
		wg.Wait()
		return nil
	}, nil
}
