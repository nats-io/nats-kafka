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

package conf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNATSClusters(t *testing.T) {
	config := DefaultBridgeConfig()
	configString := `
	nats: {
		servers: ["nats://localhost:4222"],
	}
	natsclusters: [
		{
			name: "cellB",
			servers: ["nats://cell-b:4222"],
			connecttimeout: 7000,
		},
		{
			name: "cellC",
			servers: ["nats://cell-c:4222"],
		},
	]
	connect: [
		{
			type: "NATSToKafka",
			subject: "telemetry",
			topic: "telemetry-topic",
		},
		{
			type: "NATSToKafka",
			natsconnection: "cellB",
			subject: "telemetry",
			topic: "telemetry-topic",
		},
	]
	`

	err := LoadConfigFromString(configString, &config, false)
	require.NoError(t, err)

	require.Len(t, config.NATSClusters, 2)
	require.Equal(t, "cellB", config.NATSClusters[0].Name)
	require.Equal(t, []string{"nats://cell-b:4222"}, config.NATSClusters[0].Servers)
	require.Equal(t, 7000, config.NATSClusters[0].ConnectTimeout)
	require.Equal(t, "cellC", config.NATSClusters[1].Name)

	require.Len(t, config.Connect, 2)
	require.Equal(t, "", config.Connect[0].NATSConnection)
	require.Equal(t, "cellB", config.Connect[1].NATSConnection)
}

func TestValidateNATSClustersFillsDefaults(t *testing.T) {
	config := DefaultBridgeConfig()
	config.NATS.Servers = []string{"nats://localhost:4222"}
	config.NATS.MaxReconnects = -1
	config.NATSClusters = []NATSConfig{
		{
			Name:           "cellB",
			Servers:        []string{"nats://cell-b:4222"},
			ConnectTimeout: 7000,
		},
	}

	require.NoError(t, config.ValidateNATSClusters())

	cluster := config.NATSClusters[0]
	require.Equal(t, 7000, cluster.ConnectTimeout)
	require.Equal(t, config.NATS.ClientName, cluster.ClientName)
	require.Equal(t, config.NATS.ReconnectWait, cluster.ReconnectWait)
	require.Equal(t, -1, cluster.MaxReconnects)
}

func TestValidateNATSClustersErrors(t *testing.T) {
	base := func() NATSKafkaBridgeConfig {
		config := DefaultBridgeConfig()
		config.NATS.Servers = []string{"nats://localhost:4222"}
		return config
	}

	// name on the default nats block is rejected
	config := base()
	config.NATS.Name = "oops"
	require.Error(t, config.ValidateNATSClusters())

	// missing cluster name
	config = base()
	config.NATSClusters = []NATSConfig{{Servers: []string{"nats://x:4222"}}}
	require.Error(t, config.ValidateNATSClusters())

	// the name "default" is reserved for the default nats block
	config = base()
	config.NATSClusters = []NATSConfig{{Name: "default", Servers: []string{"nats://x:4222"}}}
	require.Error(t, config.ValidateNATSClusters())

	// duplicate cluster names
	config = base()
	config.NATSClusters = []NATSConfig{
		{Name: "cellB", Servers: []string{"nats://x:4222"}},
		{Name: "cellB", Servers: []string{"nats://y:4222"}},
	}
	require.Error(t, config.ValidateNATSClusters())

	// missing servers
	config = base()
	config.NATSClusters = []NATSConfig{{Name: "cellB"}}
	require.Error(t, config.ValidateNATSClusters())

	// unknown natsconnection on a connector
	config = base()
	config.Connect = []ConnectorConfig{{Type: NATSToKafka, NATSConnection: "nope"}}
	require.Error(t, config.ValidateNATSClusters())

	// STAN connectors cannot use a named cluster
	config = base()
	config.NATSClusters = []NATSConfig{{Name: "cellB", Servers: []string{"nats://x:4222"}}}
	config.Connect = []ConnectorConfig{{Type: STANToKafka, NATSConnection: "cellB"}}
	require.Error(t, config.ValidateNATSClusters())

	// valid config passes
	config = base()
	config.NATSClusters = []NATSConfig{{Name: "cellB", Servers: []string{"nats://x:4222"}}}
	config.Connect = []ConnectorConfig{
		{Type: NATSToKafka},
		{Type: NATSToKafka, NATSConnection: "cellB"},
	}
	require.NoError(t, config.ValidateNATSClusters())
}
