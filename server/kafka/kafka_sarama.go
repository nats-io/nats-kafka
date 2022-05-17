package kafka

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
)

func GetSaramaConfig(cc conf.ConnectorConfig, clientID string, dialTimeout time.Duration) (*sarama.Config, error) {
	sc := sarama.NewConfig()
	sc.ClientID = clientID
	sc.Net.DialTimeout = dialTimeout

	if cc.SASL.Mechanism != "" {
		switch cc.SASL.Mechanism {
		case "PLAIN":
		case "SCRAM-SHA-256":
		case "SCRAM-SHA-512":
		default:
			return nil, fmt.Errorf("invalid SASL.Mechanism %s", cc.SASL.Mechanism)
		}
		sc.Net.SASL.Enable = true

		if sc.Version.IsAtLeast(sarama.V1_0_0_0) {
			sc.Net.SASL.Version = sarama.SASLHandshakeV1
		}

		sc.Net.SASL.Mechanism = (sarama.SASLMechanism)(cc.SASL.Mechanism)
		sc.Net.SASL.User = cc.SASL.User
		sc.Net.SASL.Password = cc.SASL.Password
		sc.Net.SASL.GSSAPI = cc.SASL.GSSAPI
	}

	if cc.TLS.InsecureSkipVerify {
		sc.Net.TLS.Enable = true
		sc.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: true,
		}
	} else {
		if tlsC, err := cc.TLS.MakeTLSConfig(); tlsC != nil && err == nil {
			sc.Net.TLS.Enable = true
			sc.Net.TLS.Config = tlsC
		} else {
			return nil, err
		}
	}

	return sc, nil
}
