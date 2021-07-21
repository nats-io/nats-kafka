/*
 * Copyright 2019-2020 The NATS Authors
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
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/kafka"
	gnatsserver "github.com/nats-io/nats-server/v2/server"
	gnatsd "github.com/nats-io/nats-server/v2/test"
	nss "github.com/nats-io/nats-streaming-server/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/nats-io/stan.go"
)

const (
	serverCert   = "../../resources/certs/server-cert.pem"
	serverKey    = "../../resources/certs/server-key.pem"
	clientCert   = "../../resources/certs/client-cert.pem"
	clientKey    = "../../resources/certs/client-key.pem"
	caFile       = "../../resources/certs/ca-cert.pem"
	saslUser     = "admin"
	saslPassword = "admin-secret"
)

// TestEnv encapsulate a bridge test environment
type TestEnv struct {
	Config        *conf.NATSKafkaBridgeConfig
	Gnatsd        *gnatsserver.Server
	Stan          *nss.StanServer
	KafkaHostPort string

	NC *nats.Conn            // for bypassing the bridge
	SC stan.Conn             // for bypassing the bridge
	JS nats.JetStreamContext // for bypassing the bridge

	natsPort       int
	natsURL        string
	clusterName    string
	clientID       string // we keep this so we stay the same on reconnect
	bridgeClientID string

	Bridge *NATSKafkaBridge

	useTLS bool

	useSASL  bool
	user     string
	password string
}

func collectTopics(connections []conf.ConnectorConfig) []string {
	topicSet := map[string]string{}
	topics := []string{}

	for _, c := range connections {
		if c.Topic != "" {
			topicSet[c.Topic] = c.Topic
		}
	}

	for t := range topicSet {
		topics = append(topics, t)
	}
	return topics
}

// StartTestEnvironment calls StartTestEnvironmentInfrastructure
// followed by StartBridge
func StartTestEnvironment(connections []conf.ConnectorConfig) (*TestEnv, error) {
	tbs, err := StartTestEnvironmentInfrastructure(false, false, collectTopics(connections))
	if err != nil {
		return nil, err
	}

	for _, cc := range connections {
		if !strings.Contains(cc.Type, "JetStream") {
			continue
		}
		_, err := tbs.JS.AddStream(&nats.StreamConfig{
			Name:     nuid.Next(),
			Subjects: []string{cc.Subject},
		})
		if err != nil {
			return nil, err
		}
	}

	err = tbs.StartBridge(connections)
	if err != nil {
		tbs.Close()
		return nil, err
	}
	return tbs, err
}

// StartTLSTestEnvironment calls StartTestEnvironmentInfrastructure
// followed by StartBridge, with TLS enabled
func StartTLSTestEnvironment(connections []conf.ConnectorConfig) (*TestEnv, error) {
	tbs, err := StartTestEnvironmentInfrastructure(false, true, collectTopics(connections))
	if err != nil {
		return nil, err
	}
	for _, cc := range connections {
		if !strings.Contains(cc.Type, "JetStream") {
			continue
		}
		_, err := tbs.JS.AddStream(&nats.StreamConfig{
			Name:     nuid.Next(),
			Subjects: []string{cc.Subject},
		})
		if err != nil {
			return nil, err
		}
	}
	err = tbs.StartBridge(connections)
	if err != nil {
		tbs.Close()
		return nil, err
	}
	return tbs, err
}

// StartSASLTestEnvironment calls StartTestEnvironmentInfrastructure
// followed by StartBridge, with SASL enabled
func StartSASLTestEnvironment(connections []conf.ConnectorConfig) (*TestEnv, error) {
	tbs, err := StartTestEnvironmentInfrastructure(true, false, collectTopics(connections))
	if err != nil {
		return nil, err
	}
	tbs.user = saslUser
	tbs.password = saslPassword
	for _, cc := range connections {
		if !strings.Contains(cc.Type, "JetStream") {
			continue
		}
		_, err := tbs.JS.AddStream(&nats.StreamConfig{
			Name:     nuid.Next(),
			Subjects: []string{cc.Subject},
		})
		if err != nil {
			return nil, err
		}
	}
	err = tbs.StartBridge(connections)
	if err != nil {
		tbs.Close()
		return nil, err
	}
	return tbs, err
}

// StartTestEnvironmentInfrastructure creates the kafka server, Nats and streaming
// but does not start a bridge, you can use StartBridge to start a bridge afterward
func StartTestEnvironmentInfrastructure(useSASL, useTLS bool, topics []string) (*TestEnv, error) {
	tbs := &TestEnv{}
	tbs.useTLS = useTLS
	tbs.useSASL = useSASL

	tbs.KafkaHostPort = "localhost:9092"

	if tbs.useTLS {
		tbs.KafkaHostPort = "localhost:9093"
	}

	if tbs.useSASL {
		tbs.KafkaHostPort = "localhost:9094"
		tbs.user = saslUser
		tbs.password = saslPassword
	}

	err := tbs.CheckKafka(5000)

	if err != nil {
		tbs.Close()
		return nil, err
	}

	for _, t := range topics {
		err := tbs.CreateTopic(t, 5000)
		if err != nil {
			if !kafka.IsTopicExist(err) {
				tbs.Close()
				return nil, err
			}
			// Otherwise, it's fine.
		}
	}

	err = tbs.StartNATSandStan(-1, nuid.Next(), nuid.Next(), nuid.Next())
	if err != nil {
		tbs.Close()
		return nil, err
	}

	return tbs, nil
}

// StartBridge is the second half of StartTestEnvironment
// it is provided separately so that environment can be created before the bridge runs
func (tbs *TestEnv) StartBridge(connections []conf.ConnectorConfig) error {
	config := conf.DefaultBridgeConfig()
	config.Logging.Debug = true
	config.Logging.Trace = true
	config.Logging.Colors = false
	config.Monitoring = conf.HTTPConfig{
		HTTPPort: -1,
	}
	config.NATS = conf.NATSConfig{
		Servers:        []string{tbs.natsURL},
		ConnectTimeout: 2000,
		ReconnectWait:  2000,
		MaxReconnects:  5,
	}
	config.STAN = conf.NATSStreamingConfig{
		ClusterID:          tbs.clusterName,
		ClientID:           tbs.bridgeClientID,
		PubAckWait:         5000,
		DiscoverPrefix:     stan.DefaultDiscoverPrefix,
		MaxPubAcksInflight: stan.DefaultMaxPubAcksInflight,
		ConnectWait:        2000,
	}
	config.JetStream = conf.JetStreamConfig{
		MaxWait:                5000,
		PublishAsyncMaxPending: 1,
	}

	if tbs.useTLS {
		config.Monitoring.HTTPPort = 0
		config.Monitoring.HTTPSPort = -1

		config.Monitoring.TLS = conf.TLSConf{
			Cert: serverCert,
			Key:  serverKey,
		}

		config.NATS.TLS = conf.TLSConf{
			Root: caFile,
		}
	}

	for i, c := range connections {
		c.Brokers = []string{tbs.KafkaHostPort}

		if tbs.useTLS {
			c.TLS = conf.TLSConf{
				Cert: clientCert,
				Key:  clientKey,
				Root: caFile,
			}
		}

		if tbs.useSASL {
			c.SASL = conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			}

		}

		connections[i] = c
	}

	config.Connect = connections

	tbs.Config = &config
	tbs.Bridge = NewNATSKafkaBridge()
	err := tbs.Bridge.InitializeFromConfig(config)
	if err != nil {
		tbs.Close()
		return err
	}
	err = tbs.Bridge.Start()
	if err != nil {
		tbs.Close()
		return err
	}

	// Give some time for everything to come up.
	time.Sleep(1 * time.Second)
	return nil
}

// StartNATSandStan starts up the nats and stan servers
func (tbs *TestEnv) StartNATSandStan(port int, clusterID string, clientID string, bridgeClientID string) error {
	var err error
	opts := gnatsd.DefaultTestOptions
	opts.Port = port

	if tbs.useTLS {
		opts.TLSCert = serverCert
		opts.TLSKey = serverKey
		opts.TLSTimeout = 5

		tc := gnatsserver.TLSConfigOpts{}
		tc.CertFile = opts.TLSCert
		tc.KeyFile = opts.TLSKey

		opts.TLSConfig, err = gnatsserver.GenTLSConfig(&tc)

		if err != nil {
			return err
		}
	}
	tbs.Gnatsd = gnatsd.RunServer(&opts)
	err = tbs.Gnatsd.EnableJetStream(&gnatsserver.JetStreamConfig{
		MaxMemory: 1024,
	})
	if err != nil {
		return err
	}

	if tbs.useTLS {
		tbs.natsURL = fmt.Sprintf("tls://localhost:%d", opts.Port)
	} else {
		tbs.natsURL = fmt.Sprintf("nats://localhost:%d", opts.Port)
	}

	tbs.natsPort = opts.Port
	tbs.clusterName = clusterID
	sOpts := nss.GetDefaultOptions()
	sOpts.ID = tbs.clusterName
	sOpts.NATSServerURL = tbs.natsURL

	if tbs.useTLS {
		sOpts.ClientCA = caFile
	}

	nOpts := nss.DefaultNatsServerOptions
	nOpts.Port = -1

	s, err := nss.RunServerWithOpts(sOpts, &nOpts)
	if err != nil {
		return err
	}

	tbs.Stan = s
	tbs.clientID = clientID
	tbs.bridgeClientID = bridgeClientID

	var nc *nats.Conn

	if tbs.useTLS {
		nc, err = nats.Connect(tbs.natsURL, nats.RootCAs(caFile))
	} else {
		nc, err = nats.Connect(tbs.natsURL)
	}

	if err != nil {
		return err
	}

	tbs.NC = nc

	sc, err := stan.Connect(tbs.clusterName, tbs.clientID, stan.NatsConn(tbs.NC))
	if err != nil {
		return err
	}
	tbs.SC = sc

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	tbs.JS = js

	return nil
}

// StopBridge stops the bridge
func (tbs *TestEnv) StopBridge() {
	if tbs.Bridge != nil {
		tbs.Bridge.Stop()
		tbs.Bridge = nil
	}
}

// StopNATS shuts down the NATS and Stan servers
func (tbs *TestEnv) StopNATS() error {
	if tbs.SC != nil {
		tbs.SC.Close()
	}

	if tbs.NC != nil {
		tbs.NC.Close()
	}

	if tbs.Stan != nil {
		tbs.Stan.Shutdown()
	}

	if tbs.Gnatsd != nil {
		tbs.Gnatsd.Shutdown()
	}

	return nil
}

// RestartNATS shuts down the NATS and stan server and then starts it again
func (tbs *TestEnv) RestartNATS() error {
	if tbs.SC != nil {
		tbs.SC.Close()
	}

	if tbs.NC != nil {
		tbs.NC.Close()
	}

	if tbs.Stan != nil {
		tbs.Stan.Shutdown()
	}

	if tbs.Gnatsd != nil {
		tbs.Gnatsd.Shutdown()
	}

	err := tbs.StartNATSandStan(tbs.natsPort, tbs.clusterName, tbs.clientID, tbs.bridgeClientID)
	if err != nil {
		return err
	}

	return nil
}

// Close the bridge server and clean up the test environment
func (tbs *TestEnv) Close() {
	// Stop the bridge first!
	if tbs.Bridge != nil {
		tbs.Bridge.Stop()
	}

	if tbs.SC != nil {
		tbs.SC.Close()
	}

	if tbs.NC != nil {
		tbs.NC.Close()
	}

	if tbs.Stan != nil {
		tbs.Stan.Shutdown()
	}

	if tbs.Gnatsd != nil {
		tbs.Gnatsd.Shutdown()
	}
}

// SendMessageToKafka puts a message on the kafka topic, bypassing the bridge
func (tbs *TestEnv) SendMessageToKafka(topic string, data []byte, waitMillis int32) error {
	cc := conf.ConnectorConfig{
		Brokers:   []string{tbs.KafkaHostPort},
		Partition: 0,
	}
	if tbs.useSASL {
		cc.SASL = conf.SASL{
			User:     tbs.user,
			Password: tbs.password,
		}
	}
	if tbs.useTLS {
		cc.TLS = conf.TLSConf{
			Cert: clientCert,
			Key:  clientKey,
			Root: caFile,
		}
	}

	bc := conf.NATSKafkaBridgeConfig{ConnectTimeout: int(waitMillis)}
	prod, err := kafka.NewProducer(cc, bc, topic)
	if err != nil {
		return err
	}
	defer prod.Close()

	err = prod.Write(kafka.Message{
		Value: []byte(data),
	})
	if err != nil {
		return err
	}
	return nil
}

// CreateReader creates a new reader
func (tbs *TestEnv) CreateReader(topic string, waitMillis int32) kafka.Consumer {
	cc := conf.ConnectorConfig{
		Brokers: []string{tbs.KafkaHostPort},
		Topic:   topic,
		SASL: conf.SASL{
			User:     tbs.user,
			Password: tbs.password,
		},
	}
	if tbs.useSASL {
		cc.SASL = conf.SASL{
			User:     tbs.user,
			Password: tbs.password,
		}
	}
	if tbs.useTLS {
		cc.TLS = conf.TLSConf{
			Cert: clientCert,
			Key:  clientKey,
			Root: caFile,
		}
	}

	dialTimeout := time.Duration(waitMillis) * time.Millisecond
	cons, err := kafka.NewConsumer(cc, dialTimeout)
	if err != nil {
		log.Println("failed to create consumer:", err)
		return nil
	}
	return cons
}

// GetMessageFromKafka uses an extra connection to talk to kafka, bypassing the bridge
func (tbs *TestEnv) GetMessageFromKafka(reader kafka.Consumer, waitMillis int32) ([]byte, []byte, error) {
	context, cancel := context.WithTimeout(context.Background(), time.Duration(waitMillis)*time.Millisecond)
	defer cancel()

	m, err := reader.Fetch(context)
	if err != nil {
		return nil, nil, err
	}
	if reader.GroupMode() {
		if err := reader.Commit(context, m); err != nil {
			return nil, nil, err
		}
	}

	if err != nil || m.Value == nil {
		return nil, nil, err
	}

	return m.Key, m.Value, nil
}

func (tbs *TestEnv) CreateTopic(topic string, waitMillis int32) error {
	cc := conf.ConnectorConfig{
		Brokers: []string{tbs.KafkaHostPort},
	}
	if tbs.useSASL {
		cc.SASL.User = tbs.user
		cc.SASL.Password = tbs.password
	}
	if tbs.useTLS {
		cc.TLS = conf.TLSConf{
			Cert: clientCert,
			Key:  clientKey,
			Root: caFile,
		}
	}
	bc := conf.NATSKafkaBridgeConfig{ConnectTimeout: int(waitMillis)}
	man, err := kafka.NewManager(cc, bc)
	if err != nil {
		return err
	}
	defer man.Close()

	if err := man.CreateTopic(topic, 1, 1); err != nil {
		return err
	}
	return man.Close()
}

func (tbs *TestEnv) CheckKafka(waitMillis int32) error {
	cc := conf.ConnectorConfig{
		Brokers: []string{tbs.KafkaHostPort},
	}
	if tbs.useSASL {
		cc.SASL.User = tbs.user
		cc.SASL.Password = tbs.password
	}
	if tbs.useTLS {
		cc.TLS = conf.TLSConf{
			Cert: clientCert,
			Key:  clientKey,
			Root: caFile,
		}
	}
	bc := conf.NATSKafkaBridgeConfig{ConnectTimeout: int(waitMillis)}
	man, err := kafka.NewManager(cc, bc)
	if err != nil {
		return err
	}
	return man.Close()
}

func (tbs *TestEnv) WaitForIt(requestCount int64, done chan string) string {
	timeout := time.Duration(5000) * time.Millisecond // 5 second timeout for tests
	stop := time.Now().Add(timeout)
	timer := time.NewTimer(timeout)
	requestsOk := make(chan bool)

	// Timeout the done channel
	go func() {
		<-timer.C
		done <- ""
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	go func() {
		for t := range ticker.C {
			if t.After(stop) {
				requestsOk <- false
				break
			}

			if tbs.Bridge.SafeStats().RequestCount >= requestCount {
				requestsOk <- true
				break
			}
		}
		ticker.Stop()
	}()

	received := <-done
	ok := <-requestsOk

	if !ok {
		received = ""
	}

	return received
}

func (tbs *TestEnv) WaitForRequests(requestCount int64) {
	timeout := time.Duration(5000) * time.Millisecond // 5 second timeout for tests
	stop := time.Now().Add(timeout)
	requestsOk := make(chan bool)

	ticker := time.NewTicker(50 * time.Millisecond)
	go func() {
		for t := range ticker.C {
			if t.After(stop) {
				requestsOk <- false
				break
			}

			if tbs.Bridge.SafeStats().RequestCount >= requestCount {
				requestsOk <- true
				break
			}
		}
		ticker.Stop()
	}()

	<-requestsOk
}
