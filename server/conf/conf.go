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

package conf

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/nats-io/nats-kafka/server/logging"
)

const (
	// Hash can be used in the connector config to set the hash balancing method
	Hash = "hash"
	// LeastBytes can be used in the connector config to set the least bytes received balancing method
	LeastBytes = "leastbytes"
)

const (
	// FixedKey in the keytype causes the key to always be the value in keyvalue
	FixedKey = "fixed"
	// SubjectKey in the keytype causes the key to be the incoming subject or channel
	SubjectKey = "subject"
	// ReplyToKey in the keytype causes the key to be the incoming reply-to subject or durable name
	ReplyToKey = "reply"
	// SubjectRegex in the keytype causes the key to use the keyvalue as a regex
	// on the incoming subject or channel and take the first match group as the key
	// for example foo\.(.*) for the subject/channel foo.bar would use "bar" as the key
	SubjectRegex = "subjectre"
	// ReplyRegex in the keytype causes the key to use the keyvalue as a regex
	// on the incoming reply-to and take the first match group as the key
	// for example foo\.(.*) for the subject/durable foo.bar would use "bar" as the key
	ReplyRegex = "replyre"
)

const (
	// KafkaToNATS specifies a connector from Kafka to NATS
	KafkaToNATS = "KafkaToNATS"
	// KafkaToStan specifies a connector from Kafka to NATS streaming
	KafkaToStan = "KafkaToStan"
	// KafkaToJetStream specifies a connector from Kafka to JetStream
	KafkaToJetStream = "KafkaToJetStream"

	// NATSToKafka specifies a connector from NATS to Kafka
	NATSToKafka = "NATSToKafka"
	// STANToKafka specifies a connector from NATS streaming to Kafka
	STANToKafka = "STANToKafka"
	// JetStreamToKafka specifies a connector from JetStream to Kafka
	JetStreamToKafka = "JetStreamToKafka"
)

// NATSKafkaBridgeConfig is the root structure for a bridge configuration file.
type NATSKafkaBridgeConfig struct {
	ReconnectInterval int // milliseconds
	ConnectTimeout    int // milliseconds, connect timeout for Kafka connections

	Logging      logging.Config
	NATS         NATSConfig   // the default NATS cluster (cell), shared by connectors that don't name one
	NATSClusters []NATSConfig // optional additional named NATS clusters (cells)
	STAN         NATSStreamingConfig
	JetStream    JetStreamConfig
	Monitoring   HTTPConfig
	Connect      []ConnectorConfig
}

// TLSConf holds the configuration for a TLS connection/server
type TLSConf struct {
	Key  string
	Cert string
	Root string
}

// SASL holds the configuration for SASL authentication with Kafka.
type SASL struct {
	User               string
	Password           string
	InsecureSkipVerify bool
	Mechanism          string
	TLS                bool
}

type IAM struct {
	Enable bool
	Region string
}

// MakeTLSConfig creates a tls.Config from a TLSConf, setting up the key pairs and certs
func (tlsConf *TLSConf) MakeTLSConfig() (*tls.Config, error) {
	if tlsConf.Cert == "" || tlsConf.Key == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(tlsConf.Cert, tlsConf.Key)
	if err != nil {
		return nil, fmt.Errorf("error loading X509 certificate/key pair: %v", err)
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate: %v", err)
	}

	config := tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		ClientAuth:               tls.NoClientCert,
		PreferServerCipherSuites: true,
	}

	if tlsConf.Root != "" {
		// Load CA cert
		caCert, err := os.ReadFile(tlsConf.Root)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		config.RootCAs = caCertPool
	}

	return &config, nil
}

// HTTPConfig is used to specify the host/port/tls for an HTTP server
type HTTPConfig struct {
	HTTPHost  string
	HTTPPort  int
	HTTPSPort int
	TLS       TLSConf

	ReadTimeout  int // milliseconds
	WriteTimeout int // milliseconds

	// DisableMetrics turns off the Prometheus /metrics endpoint,
	// which is otherwise served whenever monitoring is enabled.
	DisableMetrics bool
	// DisableDashboard turns off the /dashboard page,
	// which is otherwise served whenever monitoring is enabled.
	DisableDashboard bool
}

// NATSConfig configuration for a NATS connection
type NATSConfig struct {
	Name           string // cluster (cell) name, required for entries in natsclusters, must be empty on the default nats block
	Servers        []string
	ClientName     string // optional client name
	ConnectTimeout int    // milliseconds
	ReconnectWait  int    // milliseconds
	MaxReconnects  int

	TLS             TLSConf
	UserCredentials string
	UserNKEY        string
}

// NATSStreamingConfig configuration for a STAN connection
type NATSStreamingConfig struct {
	ClusterID string
	ClientID  string

	PubAckWait         int // milliseconds
	DiscoverPrefix     string
	MaxPubAcksInflight int
	ConnectWait        int // milliseconds
}

// JetStreamConfig configuration for a JetStream connection
type JetStreamConfig struct {
	PublishAsyncMaxPending int
	MaxWait                int // milliseconds
	EnableFlowControl      bool
	EnableAckSync          bool
	HeartbeatInterval      int // milliseconds
}

// DefaultBridgeConfig generates a default configuration with
// logging set to colors, time, debug and trace
func DefaultBridgeConfig() NATSKafkaBridgeConfig {
	return NATSKafkaBridgeConfig{
		ReconnectInterval: 5000,
		ConnectTimeout:    10000,
		Logging: logging.Config{
			Colors: true,
			Time:   true,
			Debug:  false,
			Trace:  false,
		},
		Monitoring: HTTPConfig{
			ReadTimeout:  5000,
			WriteTimeout: 5000,
		},
		NATS: NATSConfig{
			ClientName:     "NATS Kafka Bridge",
			ConnectTimeout: 5000,
			ReconnectWait:  1000,
			MaxReconnects:  0,
		},
	}
}

// ValidateNATSClusters fills defaults on the named NATS cluster entries and
// checks the cluster/connector wiring. Entries in NATSClusters come from the
// reflection-based parser zero-valued, so the defaults applied to the default
// nats block in DefaultBridgeConfig are applied here for each entry.
func (bridge *NATSKafkaBridgeConfig) ValidateNATSClusters() error {
	if bridge.NATS.Name != "" {
		return fmt.Errorf("the default nats block must not set a name, found %q", bridge.NATS.Name)
	}

	names := map[string]bool{}
	for i := range bridge.NATSClusters {
		cluster := &bridge.NATSClusters[i]

		if cluster.Name == "" {
			return fmt.Errorf("natsclusters entry %d is missing a name", i)
		}
		if cluster.Name == "default" {
			return fmt.Errorf("natsclusters name %q is reserved for the default nats block", cluster.Name)
		}
		if names[cluster.Name] {
			return fmt.Errorf("duplicate natsclusters name %q", cluster.Name)
		}
		names[cluster.Name] = true

		if len(cluster.Servers) == 0 {
			return fmt.Errorf("natsclusters entry %q has no servers", cluster.Name)
		}

		if cluster.ClientName == "" {
			cluster.ClientName = bridge.NATS.ClientName
		}
		if cluster.ConnectTimeout == 0 {
			cluster.ConnectTimeout = bridge.NATS.ConnectTimeout
		}
		if cluster.ReconnectWait == 0 {
			cluster.ReconnectWait = bridge.NATS.ReconnectWait
		}
		if cluster.MaxReconnects == 0 {
			cluster.MaxReconnects = bridge.NATS.MaxReconnects
		}
	}

	for _, c := range bridge.Connect {
		if c.NATSConnection == "" {
			continue
		}
		if !names[c.NATSConnection] {
			return fmt.Errorf("connector %q (type %q) references unknown natsconnection %q",
				c.ID, c.Type, c.NATSConnection)
		}
		if c.Type == STANToKafka || c.Type == KafkaToStan {
			return fmt.Errorf("connector %q (type %q) cannot set natsconnection, STAN connectors only support the default nats connection",
				c.ID, c.Type)
		}
	}

	return nil
}

// ConnectorConfig configuration for a bridge connection (of any type)
type ConnectorConfig struct {
	ID   string // user specified id for a connector, will be defaulted if none is provided
	Type string // Can be any of the type constants (STANToKafka, ...)

	NATSConnection string // optional name of a cluster from natsclusters, empty means the default nats block

	Channel         string // Used for stan connections
	DurableName     string // Optional, used for stan and jetstream connections
	StartAtSequence int64  // Start position for stan and jetstream connection, -1 means StartWithLastReceived, 0 means DeliverAllAvailable (default)
	StartAtTime     int64  // Start time, as Unix, time takes precedence over sequence

	Subject   string // Used for nats and jetstream connections
	QueueName string // Optional, used for nats connections
	Stream    string // Uses BindStream option for JetStream to consume from sourced streams

	Brokers []string // list of brokers to use for creating a reader/writer
	Topic   string   // kafka topic
	TLS     TLSConf  // tls config for connecting to the kafka brokers
	SASL    SASL     // SASL config for connecting to the kafka brokers, specifically EventHub
	IAM     IAM      // IAM config for connecting to kafka brokers

	MinBytes  int64  // used by the Kafka reader (for kafka->nats connectors)
	MaxBytes  int64  // used by the Kafka reader (for kafka->nats connectors)
	Partition int64  // optional partition for the reader
	GroupID   string // optional group id for reader, exclusive with partition

	Balancer string // can be hash or leastbytes

	KeyType  string // what to do with the key, can be FixedKey, ...
	KeyValue string // extra data for handling the key based on the keytype, may be ignored

	SchemaRegistryURL string // Schema registry url for message schema validation
	SubjectName       string // Name of the subject in the schema registry for the value
	SchemaVersion     int    // Version of the value schema to use. Default is latest.
	SchemaType        string // Can be avro, json, protobuf. Default is avro.
}
