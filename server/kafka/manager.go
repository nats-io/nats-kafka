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
