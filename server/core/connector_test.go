package core

import (
	"fmt"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/logging"
	"github.com/stretchr/testify/require"
)

func TestUnknownConnector(t *testing.T) {
	_, err := CreateConnector(conf.ConnectorConfig{
		Type: "foo",
	}, nil)
	require.Error(t, err)
}

func TestNoOp(t *testing.T) {
	conn := &BridgeConnector{}
	require.NoError(t, conn.Start())
	require.NoError(t, conn.Shutdown())
	require.NoError(t, conn.CheckConnections())
}

func TestCalculateKeyInvalidKeyValue(t *testing.T) {
	conn := &BridgeConnector{
		config: conf.ConnectorConfig{
			KeyValue: "[[[",
		},
		bridge: &NATSKafkaBridge{
			logger: logging.NewNATSLogger(logging.Config{}),
		},
		stats: NewConnectorStatsHolder("name", "id"),
	}

	conn.config.KeyType = conf.SubjectRegex
	require.Equal(t, conn.calculateKey("subject", "replyto"), []byte{})

	conn.config.KeyType = conf.ReplyRegex
	require.Equal(t, conn.calculateKey("subject", "replyto"), []byte{})
}

func TestConvertFromNatsToKafkaHeadersOk(t *testing.T) {
	conn := &BridgeConnector{
		config: conf.ConnectorConfig{
			KeyValue: "[[[",
		},
		bridge: &NATSKafkaBridge{
			logger: logging.NewNATSLogger(logging.Config{}),
		},
		stats: NewConnectorStatsHolder("name", "id"),
	}

	var natsHdr = make(nats.Header)
	natsHdr.Add("a", "aaa")
	natsHdr.Add("b", "bbb")
	natsHdr.Add("c", "ccc")

	var kHdrs = conn.convertFromNatsToKafkaHeaders(natsHdr)
	require.Equal(t, 3, len(kHdrs))
	for key, element := range kHdrs {
		require.Equal(t, natsHdr.Get(string(element.Key)), string(element.Value))
		key++
	}
}

func TestConvertFromNatsToKafkaHeadersNull(t *testing.T) {
	conn := &BridgeConnector{
		config: conf.ConnectorConfig{
			KeyValue: "[[[",
		},
		bridge: &NATSKafkaBridge{
			logger: logging.NewNATSLogger(logging.Config{}),
		},
		stats: NewConnectorStatsHolder("name", "id"),
	}

	//kafkaHdrs := make([]sarama.RecordHeader, len(natsHdr))
	var kHdrs = conn.convertFromNatsToKafkaHeaders(nil)
	require.Equal(t, kHdrs, []sarama.RecordHeader{})
}

func TestConvertFromKafkaToNatsHeadersOk(t *testing.T) {
	conn := &BridgeConnector{
		config: conf.ConnectorConfig{
			KeyValue: "[[[",
		},
		bridge: &NATSKafkaBridge{
			logger: logging.NewNATSLogger(logging.Config{}),
		},
		stats: NewConnectorStatsHolder("name", "id"),
	}

	var kHdrs = make([]sarama.RecordHeader, 3)
	i := 0
	for i < len(kHdrs) {
		keyString := fmt.Sprintf("key-%d", i)
		valueString := fmt.Sprintf("value-%d", i)
		kHdrs[i].Key = []byte(keyString)
		kHdrs[i].Value = []byte(valueString)
		i++
	}

	var nHdrs = conn.convertFromKafkaToNatsHeaders(kHdrs)
	i = 0
	require.Equal(t, 3, len(nHdrs))
	for nKey, nValue := range nHdrs {
		require.Equal(t, string(kHdrs[i].Key), nKey)
		require.Equal(t, string(kHdrs[i].Value), nValue[0])
		i++
	}
}

func TestConvertFromKafkaToNatsHeadersNull(t *testing.T) {
	conn := &BridgeConnector{
		config: conf.ConnectorConfig{
			KeyValue: "[[[",
		},
		bridge: &NATSKafkaBridge{
			logger: logging.NewNATSLogger(logging.Config{}),
		},
		stats: NewConnectorStatsHolder("name", "id"),
	}

	var nHdrs = conn.convertFromKafkaToNatsHeaders(nil)
	require.Equal(t, nHdrs, nats.Header{})
}
