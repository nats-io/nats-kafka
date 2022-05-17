package kafka

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"fmt"
	"hash"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/xdg-go/scram"
)

// Predefined SCRAMClientGeneratorFunc, copied from https://github.com/Shopify/sarama/blob/master/examples/sasl_scram_client/scram_client.go

var SHA256 scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }
var SHA512 scram.HashGeneratorFcn = func() hash.Hash { return sha512.New() }

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}


// GetSaramaConfig creates a common sarama.Config
func GetSaramaConfig(cc conf.ConnectorConfig, clientID string, dialTimeout time.Duration) (*sarama.Config, error) {
	sc := sarama.NewConfig()
	sc.ClientID = clientID
	sc.Net.DialTimeout = dialTimeout

	if cc.SASL.Mechanism != "" {
		switch strings.ToUpper(cc.SASL.Mechanism) {
		case "PLAIN":
			sc.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		case "SCRAM-SHA-256":
			sc.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			sc.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		case "SCRAM-SHA-512":
			sc.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			sc.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		default:
			return nil, fmt.Errorf("invalid SASL.Mechanism %s", cc.SASL.Mechanism)
		}
		sc.Net.SASL.Enable = true

		if sc.Version.IsAtLeast(sarama.V1_0_0_0) {
			sc.Net.SASL.Version = sarama.SASLHandshakeV1
		}

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
