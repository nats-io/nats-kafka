package core

import (
	"testing"

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
