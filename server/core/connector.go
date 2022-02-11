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
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Shopify/sarama"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/kafka"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/nats-io/stan.go"
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

func validateStanConfig(config *conf.ConnectorConfig) error {
	if config.Channel == "" {
		return fmt.Errorf("STAN channel name not specified for config type %q - ID %q",
			config.Type, config.ID)
	}
	return nil
}

// CreateConnector builds a connector from the supplied configuration
func CreateConnector(config conf.ConnectorConfig, bridge *NATSKafkaBridge) (Connector, error) {
	switch config.Type {
	case conf.NATSToKafka:
		return NewNATS2KafkaConnector(bridge, config), nil
	case conf.STANToKafka:
		if err := validateStanConfig(&config); err != nil {
			return nil, err
		}
		return NewStan2KafkaConnector(bridge, config), nil
	case conf.JetStreamToKafka:
		return NewJetStream2KafkaConnector(bridge, config), nil

	case conf.KafkaToNATS:
		return NewKafka2NATSConnector(bridge, config), nil
	case conf.KafkaToStan:
		if err := validateStanConfig(&config); err != nil {
			return nil, err
		}
		return NewKafka2StanConnector(bridge, config), nil
	case conf.KafkaToJetStream:
		return NewKafka2JetStreamConnector(bridge, config), nil
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

	config       conf.ConnectorConfig
	bridge       *NATSKafkaBridge
	stats        *ConnectorStatsHolder
	writers      *sync.Map
	destTemplate *template.Template
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
func (conn *BridgeConnector) init(bridge *NATSKafkaBridge, config conf.ConnectorConfig, destTpl string, name string) {
	conn.config = config
	conn.bridge = bridge
	conn.writers = &sync.Map{}

	id := conn.config.ID
	if id == "" {
		id = nuid.Next()
	}
	conn.stats = NewConnectorStatsHolder(name, id)

	conn.initDestTemplate(destTpl)
}

// NATSCallback used by conn-nats connectors in an conn library callback
// The lock will be held by the caller!
type NATSCallback func(msg kafka.Message) error

// ShutdownCallback is returned when setting up a callback or polling so the connector can shut it down
type ShutdownCallback func() error

func (conn *BridgeConnector) jetStreamMessageHandler(msg kafka.Message) error {
	nMsg := nats.NewMsg(conn.dest(msg))
	nMsg.Header = conn.convertFromKafkaToNatsHeaders(msg.Headers)
	nMsg.Data = msg.Value
	_, err := conn.bridge.JetStream().PublishMsg(nMsg)
	return err
}

func (conn *BridgeConnector) stanMessageHandler(msg kafka.Message) error {
	return conn.bridge.Stan().Publish(conn.dest(msg), msg.Value)
}

func (conn *BridgeConnector) natsMessageHandler(msg kafka.Message) error {
	nMsg := nats.NewMsg(conn.dest(msg))
	nMsg.Header = conn.convertFromKafkaToNatsHeaders(msg.Headers)
	nMsg.Data = msg.Value
	return conn.bridge.NATS().PublishMsg(nMsg)
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

func (conn *BridgeConnector) convertFromKafkaToNatsHeaders(hdrs []sarama.RecordHeader) nats.Header {
	// Iterate over all keys
	if len(hdrs) > 0 {
		nHdrs := make(nats.Header)
		for _, kHdr := range hdrs {
			if kHdr.Value != nil {
				nHdrs.Add(string(kHdr.Key), string(kHdr.Value))
			}
		}
		return nHdrs
	}
	return nats.Header{} // empty header by default
}

func (conn *BridgeConnector) convertFromNatsToKafkaHeaders(hdr nats.Header) []sarama.RecordHeader {
	// Iterate over all keys
	if len(hdr) > 0 {
		kHdrs := make([]sarama.RecordHeader, len(hdr))

		i := 0
		for k, v := range hdr {
			kHdrs[i].Key = []byte(k)
			if v != nil {
				kHdrs[i].Value = []byte(v[0])
			}
			i++
		}
		return kHdrs
	}
	return []sarama.RecordHeader{} // empty header by default
}

// set up a nats subscription, assumes the lock is held
func (conn *BridgeConnector) subscribeToNATS(subject string, queueName string) (*nats.Subscription, error) {
	traceEnabled := conn.bridge.Logger().TraceEnabled()
	callback := func(msg *nats.Msg) {
		start := time.Now()
		l := int64(len(msg.Data))

		// send to kafka here
		err := conn.writer(msg).Write(kafka.Message{
			Key:     conn.calculateKey(msg.Subject, msg.Reply),
			Value:   msg.Data,
			Headers: conn.convertFromNatsToKafkaHeaders(msg.Header),
		})

		if err != nil {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s wrote message to kafka", conn.String())
			}
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Errorf("connector publish failure, %s, %s", conn.String(), err.Error())
		} else {
			conn.stats.AddRequest(l, l, time.Since(start))
		}
	}

	if conn.bridge.NATS() == nil {
		return nil, fmt.Errorf("bridge not configured to use NATS streaming")
	}

	if queueName == "" {
		return conn.bridge.NATS().Subscribe(subject, callback)
	}

	return conn.bridge.NATS().QueueSubscribe(subject, queueName, callback)
}

// subscribeToChannel uses the bridges STAN connection to subscribe based on
// the config The start position/time and durable name are optional
func (conn *BridgeConnector) subscribeToChannel() (stan.Subscription, error) {
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
		err := conn.writer(msg).Write(kafka.Message{
			Key:   key,
			Value: msg.Data,
		})

		if err != nil {
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Errorf("connector publish failure, %s, %s", conn.String(), err.Error())
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

// set up a JetStream subscription, assumes the lock is held
func (conn *BridgeConnector) subscribeToJetStream(subject string) (*nats.Subscription, error) {
	if conn.bridge.JetStream() == nil {
		return nil, fmt.Errorf("bridge not configured to use JetStream")
	}

	options := []nats.SubOpt{}

	if conn.config.DurableName != "" {
		options = append(options, nats.Durable(conn.config.DurableName))
	}

	if conn.config.StartAtTime != 0 {
		t := time.Unix(conn.config.StartAtTime, 0)
		options = append(options, nats.StartTime(t))
	} else if conn.config.StartAtSequence == -1 {
		options = append(options, nats.DeliverLast())
	} else if conn.config.StartAtSequence > 0 {
		options = append(options, nats.StartSequence(uint64(conn.config.StartAtSequence)))
	} else {
		options = append(options, nats.DeliverAll())
	}

	options = append(options, nats.AckExplicit())

	if conn.bridge.config.JetStream.EnableFlowControl {
		options = append(options, nats.EnableFlowControl())
	}
	if d := conn.bridge.config.JetStream.HeartbeatInterval; d > 0 {
		options = append(options, nats.IdleHeartbeat(time.Duration(d)*time.Millisecond))
	}

	traceEnabled := conn.bridge.Logger().TraceEnabled()
	ackSyncEnabled := conn.bridge.config.JetStream.EnableAckSync

	callback := func(msg *nats.Msg) {
		start := time.Now()
		l := int64(len(msg.Data))

		if traceEnabled {
			conn.bridge.Logger().Tracef("%s received message", conn.String())
		}

		key := conn.calculateKey(conn.config.Subject, conn.config.DurableName)
		err := conn.writer(msg).Write(kafka.Message{
			Key:     key,
			Value:   msg.Data,
			Headers: conn.convertFromNatsToKafkaHeaders(msg.Header),
		})

		if err != nil {
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Errorf("connector publish failure, %s, %s", conn.String(), err.Error())
		} else {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s wrote message to kafka with key %s", conn.String(), string(key))
			}
			if ackSyncEnabled {
				msg.AckSync()
			} else {
				msg.Ack()
			}
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s acked message to kafka", conn.String())
			}
			conn.stats.AddRequest(l, l, time.Since(start))
		}
	}

	return conn.bridge.JetStream().Subscribe(subject, callback, options...)
}

func (conn *BridgeConnector) setUpListener(target kafka.Consumer, natsCallbackFunc NATSCallback) (ShutdownCallback, error) {
	done := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(1)

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	traceEnabled := conn.bridge.Logger().TraceEnabled()

	listenerCallbackFunc := func(conn *BridgeConnector, msg kafka.Message) {
		start := time.Now()
		l := int64(len(msg.Value))
		err := natsCallbackFunc(msg)

		if err != nil {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s received message from kafka", conn.String())
			}
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Errorf("publish failure for %s, %s", conn.String(), err.Error())
			return
		}

		if conn.config.GroupID != "" {
			err := target.Commit(cancelCtx, msg)

			if err != nil {
				conn.stats.AddMessageIn(l)
				conn.bridge.Logger().Errorf("failed to commit, %s", err.Error())
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
			msg, err := target.Fetch(cancelCtx)

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

func (conn *BridgeConnector) writer(msg interface{}) kafka.Producer {
	t := conn.dest(msg)
	h := getHash(t)
	w, ok := conn.writers.Load(h)
	if !ok {
		sp, err := kafka.NewProducer(conn.config, conn.bridge.config, t)
		if err != nil {
			w = kafka.NewErroredProducer(fmt.Errorf("failed to create producer: %w", err))
		} else {
			w = sp
		}
		if s, ok := w.(interface{ NetInfo() string }); ok {
			conn.bridge.Logger().Noticef(s.NetInfo())
		}

		conn.writers.Store(h, w)
	}
	return w.(kafka.Producer)
}

func (conn *BridgeConnector) closeWriters() {
	conn.writers.Range(func(t, w interface{}) bool {
		w.(io.Closer).Close()
		conn.writers.Delete(t.(string))
		return true
	})
}

func (conn *BridgeConnector) initDestTemplate(destTpl string) {
	funcMap := template.FuncMap{
		"replace": func(old, new, src string) string {
			return strings.Replace(src, old, new, -1)
		},
		"substring": func(start, end int, s string) string {
			if start < 0 {
				return s[:end]
			}
			if end < 0 || end > len(s) {
				return s[start:]
			}
			return s[start:end]
		},
	}
	var err error
	conn.destTemplate, err = template.New("dest").Funcs(funcMap).Parse(destTpl)
	if err != nil {
		conn.bridge.logger.Fatalf("parsing destination (subject, channel, topic) went wrong: %s", err)
	}
}

func (conn *BridgeConnector) dest(msg interface{}) string {
	var buf bytes.Buffer
	if err := conn.destTemplate.Execute(&buf, msg); err != nil {
		return ""
	}
	return buf.String()
}

func getHash(name string) string {
	digits := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	sha := sha256.New()
	sha.Write([]byte(name))
	b := sha.Sum(nil)
	for i := 0; i < 6; i++ {
		b[i] = digits[int(b[i]%62)]
	}
	return string(b[:6])
}
