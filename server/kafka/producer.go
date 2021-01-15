package kafka

import (
	"errors"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
)

type Producer interface {
	Write(Message) error
	Close() error
}

type saramaProducer struct {
	sp    sarama.SyncProducer
	topic string
}

func IsTopicExist(err error) bool {
	var terr *sarama.TopicError
	if !errors.As(err, &terr) {
		return false
	}
	return terr.Err == sarama.ErrTopicAlreadyExists
}

func NewProducer(cc conf.ConnectorConfig, dialTimeout time.Duration, topic string) (Producer, error) {
	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	sc.Net.DialTimeout = dialTimeout
	sc.ClientID = "nats-kafka-producer"

	if cc.SASL.User != "" {
		sc.Net.SASL.Enable = true
		sc.Net.SASL.User = cc.SASL.User
		sc.Net.SASL.Password = cc.SASL.Password
	} else if tlsC, err := cc.TLS.MakeTLSConfig(); err == nil {
		sc.Net.TLS.Enable = (tlsC != nil)
		sc.Net.TLS.Config = tlsC
	}

	sp, err := sarama.NewSyncProducer(cc.Brokers, sc)
	if err != nil {
		return nil, err
	}

	return &saramaProducer{sp: sp, topic: topic}, nil
}

func (p *saramaProducer) Write(m Message) error {
	_, _, err := p.sp.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.StringEncoder(m.Value),
		Key:   sarama.StringEncoder(m.Key),
	})
	return err
}

func (p *saramaProducer) Close() error {
	return p.sp.Close()
}

type erroredProducer struct {
	err error
}

func NewErroredProducer(err error) Producer {
	return &erroredProducer{err: err}
}

func (p *erroredProducer) Write(m Message) error {
	return p.err
}

func (p *erroredProducer) Close() error {
	return p.err
}
