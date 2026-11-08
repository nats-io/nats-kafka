/*
 * Copyright 2026 The NATS Authors
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
	"testing"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/require"
)

func readMessagesFromTopic(t *testing.T, tbs *TestEnv, topic string, count int) []string {
	t.Helper()
	reader := tbs.CreateReader(topic, 5000)
	require.NotNil(t, reader)
	defer reader.Close()

	received := []string{}
	for i := 0; i < count; i++ {
		_, data, _, err := tbs.GetMessageFromKafka(reader, 10000)
		require.NoError(t, err)
		received = append(received, string(data))
	}
	return received
}

func TestMultiCellNatsToKafka(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
		},
		{
			Type:           "NATSToKafka",
			NATSConnection: TestCellName,
			Subject:        subject,
			Topic:          topic,
		},
	}

	tbs, err := StartTwoCellTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	require.NoError(t, tbs.NC.Publish(subject, []byte("from-default-cell")))
	require.NoError(t, tbs.NC2.Publish(subject, []byte("from-second-cell")))

	received := readMessagesFromTopic(t, tbs, topic, 2)
	require.ElementsMatch(t, []string{"from-default-cell", "from-second-cell"}, received)

	// both cells should be visible and connected in the bridge stats
	stats := tbs.Bridge.SafeStats()
	require.Len(t, stats.NATS, 2)
	require.Equal(t, TestCellName, stats.NATS[0].Name)
	require.True(t, stats.NATS[0].Connected)
	require.Equal(t, "default", stats.NATS[1].Name)
	require.True(t, stats.NATS[1].Connected)

	// the second cell's connector is labeled with its cluster
	names := map[string]string{}
	for _, c := range stats.Connections {
		names[c.Name] = c.NATSConnection
	}
	require.Contains(t, names, "NATS:"+subject+" to Kafka:"+topic)
	require.Contains(t, names, "["+TestCellName+"] NATS:"+subject+" to Kafka:"+topic)
	require.Equal(t, TestCellName, names["["+TestCellName+"] NATS:"+subject+" to Kafka:"+topic])
}

func TestMultiCellJetStreamToKafka(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: subject,
			Topic:   topic,
		},
		{
			Type:           "JetStreamToKafka",
			NATSConnection: TestCellName,
			Subject:        subject,
			Topic:          topic,
		},
	}

	tbs, err := StartTwoCellTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(subject, []byte("from-default-cell"))
	require.NoError(t, err)
	_, err = tbs.JS2.Publish(subject, []byte("from-second-cell"))
	require.NoError(t, err)

	received := readMessagesFromTopic(t, tbs, topic, 2)
	require.ElementsMatch(t, []string{"from-default-cell", "from-second-cell"}, received)
}

func TestMultiCellFailureIsolation(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
		},
		{
			Type:           "NATSToKafka",
			NATSConnection: TestCellName,
			Subject:        subject,
			Topic:          topic,
		},
	}

	tbs, err := StartTwoCellTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.reconnectLock.Lock()
	tbs.Bridge.config.ReconnectInterval = 250
	tbs.Bridge.reconnectLock.Unlock()

	// take the second cell down and wait for the bridge to notice
	tbs.StopSecondNATS()
	require.Eventually(t, func() bool {
		stats := tbs.Bridge.SafeStats()
		for _, s := range stats.NATS {
			if s.Name == TestCellName {
				return !s.Connected
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond, "second cell should show as disconnected")

	// the default cell's connector must keep delivering
	require.NoError(t, tbs.NC.Publish(subject, []byte("during-outage")))
	received := readMessagesFromTopic(t, tbs, topic, 1)
	require.Equal(t, []string{"during-outage"}, received)

	// bring the second cell back and wait for its connector to recover
	require.NoError(t, tbs.RestartSecondNATS())
	require.Eventually(t, func() bool {
		stats := tbs.Bridge.SafeStats()
		for _, s := range stats.NATS {
			if s.Name == TestCellName && s.Connected {
				for _, c := range stats.Connections {
					if c.NATSConnection == TestCellName {
						return c.Connected
					}
				}
			}
		}
		return false
	}, 20*time.Second, 100*time.Millisecond, "second cell connector should recover")

	require.NoError(t, tbs.NC2.Publish(subject, []byte("after-recovery")))
	received = readMessagesFromTopic(t, tbs, topic, 2)
	require.Contains(t, received, "after-recovery")
}

func TestMultiCellUnreachableCellAtStartup(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
		},
		{
			Type:           "NATSToKafka",
			NATSConnection: TestCellName,
			Subject:        subject,
			Topic:          topic,
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(false, false, collectTopics(connect))
	require.NoError(t, err)
	defer tbs.Close()

	// point the second cell at a port nothing listens on, the bridge must
	// start anyway and keep the default cell's connector working
	tbs.natsURL2 = "nats://localhost:59999"
	require.NoError(t, tbs.StartBridge(connect))

	stats := tbs.Bridge.SafeStats()
	require.Len(t, stats.NATS, 2)
	for _, s := range stats.NATS {
		if s.Name == TestCellName {
			require.False(t, s.Connected)
		} else {
			require.True(t, s.Connected)
		}
	}

	require.NoError(t, tbs.NC.Publish(subject, []byte("default-cell-works")))
	received := readMessagesFromTopic(t, tbs, topic, 1)
	require.Equal(t, []string{"default-cell-works"}, received)
}

func TestMultiCellMisconfiguredConnectorOnReachableCell(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: subject,
			Topic:   topic,
		},
		{
			// the second cell is reachable, but the connector binds a stream
			// that does not exist there, e.g. a wrong cell id in its name
			Type:           "JetStreamToKafka",
			NATSConnection: TestCellName,
			Subject:        subject,
			Topic:          topic,
			Stream:         "no-such-stream",
		},
	}

	// the bridge must start and keep the default cell's connector working
	tbs, err := StartTwoCellTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	stats := tbs.Bridge.SafeStats()
	for _, s := range stats.NATS {
		require.True(t, s.Connected, "cluster %s should be connected", s.Name)
	}
	for _, c := range stats.Connections {
		if c.NATSConnection == TestCellName {
			require.False(t, c.Connected, "misconfigured connector should not be running")
		} else {
			require.True(t, c.Connected, "default cell connector should be running")
		}
	}

	tbs.Bridge.reconnectLock.Lock()
	require.Len(t, tbs.Bridge.reconnect, 1, "misconfigured connector should be queued for retry")
	tbs.Bridge.reconnectLock.Unlock()

	_, err = tbs.JS.Publish(subject, []byte("default-cell-works"))
	require.NoError(t, err)
	received := readMessagesFromTopic(t, tbs, topic, 1)
	require.Equal(t, []string{"default-cell-works"}, received)
}

func TestMultiCellUnknownClusterFailsStart(t *testing.T) {
	config := conf.DefaultBridgeConfig()
	config.Logging.Colors = false
	config.NATS.Servers = []string{"nats://localhost:4222"}
	config.Connect = []conf.ConnectorConfig{
		{
			Type:           "NATSToKafka",
			NATSConnection: "no-such-cell",
			Subject:        "subject",
			Topic:          "topic",
		},
	}

	bridge := NewNATSKafkaBridge()
	require.NoError(t, bridge.InitializeFromConfig(config))
	err := bridge.Start()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-cell")
	bridge.Stop()
}

func TestMultiCellStanConnectorFailsStart(t *testing.T) {
	config := conf.DefaultBridgeConfig()
	config.Logging.Colors = false
	config.NATS.Servers = []string{"nats://localhost:4222"}
	config.NATSClusters = []conf.NATSConfig{
		{
			Name:    "cellB",
			Servers: []string{"nats://localhost:4223"},
		},
	}
	config.Connect = []conf.ConnectorConfig{
		{
			Type:           "STANToKafka",
			NATSConnection: "cellB",
			Channel:        "channel",
			Topic:          "topic",
		},
	}

	bridge := NewNATSKafkaBridge()
	require.NoError(t, bridge.InitializeFromConfig(config))
	err := bridge.Start()
	require.Error(t, err)
	require.Contains(t, err.Error(), "STAN")
	bridge.Stop()
}
