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
 *
 */

package core

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/require"
)

func TestMetricsEndpoint(t *testing.T) {
	subject := "test"
	topic := nuid.Next()

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	client := http.Client{}
	response, err := client.Get(tbs.Bridge.GetMonitoringRootURL() + "metrics")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, response.Header.Get("Content-Type"), "text/plain")

	contents, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	metrics := string(contents)

	require.Contains(t, metrics, "nats_kafka_bridge_info{version=")
	require.Contains(t, metrics, "nats_kafka_bridge_start_time_seconds")
	require.Contains(t, metrics, `nats_kafka_bridge_nats_connected{cluster="default"} 1`)
	require.Contains(t, metrics, `nats_kafka_bridge_nats_in_bytes_total{cluster="default"}`)
	require.Contains(t, metrics, `nats_kafka_bridge_nats_out_bytes_total{cluster="default"}`)
	require.Contains(t, metrics, `nats_kafka_bridge_http_requests_total{path="/metrics"}`)
	require.Contains(t, metrics, `nats_kafka_bridge_connector_connected{connector="NATS:`+subject+` to Kafka:`+topic+`"} 1`)
	require.Contains(t, metrics, "nats_kafka_bridge_connector_messages_in_total")
	require.Contains(t, metrics, "nats_kafka_bridge_connector_request_duration_seconds")

	// push a message through and check the counters move
	tbs.Bridge.checkConnections()
	err = tbs.NC.Publish(subject, []byte("hello world"))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)

	response, err = client.Get(tbs.Bridge.GetMonitoringRootURL() + "metrics")
	require.NoError(t, err)
	defer response.Body.Close()
	contents, err = io.ReadAll(response.Body)
	require.NoError(t, err)
	metrics = string(contents)

	require.Contains(t, metrics, `nats_kafka_bridge_connector_messages_in_total{connector="NATS:`+subject+` to Kafka:`+topic+`"} 1`)
	require.Contains(t, metrics, `nats_kafka_bridge_connector_messages_out_total{connector="NATS:`+subject+` to Kafka:`+topic+`"} 1`)
}

func TestDashboardEndpoint(t *testing.T) {
	tbs, err := StartTestEnvironment([]conf.ConnectorConfig{})
	require.NoError(t, err)
	defer tbs.Close()

	client := http.Client{}
	response, err := client.Get(tbs.Bridge.GetMonitoringRootURL() + "dashboard")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, response.Header.Get("Content-Type"), "text/html")

	contents, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(contents), "nats-kafka-bridge-dashboard")

	stats := tbs.Bridge.SafeStats()
	require.Equal(t, int64(1), stats.HTTPRequests[DashboardPath])
}

func TestVarzNATSStats(t *testing.T) {
	tbs, err := StartTestEnvironment([]conf.ConnectorConfig{})
	require.NoError(t, err)
	defer tbs.Close()

	client := http.Client{}
	response, err := client.Get(tbs.Bridge.GetMonitoringRootURL() + "varz")
	require.NoError(t, err)
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	bridgeStats := BridgeStats{}
	err = json.Unmarshal(contents, &bridgeStats)
	require.NoError(t, err)

	require.Len(t, bridgeStats.NATS, 1)
	require.Equal(t, "default", bridgeStats.NATS[0].Name)
	require.True(t, bridgeStats.NATS[0].Connected)
	require.NotEmpty(t, bridgeStats.NATS[0].ConnectedURL)
}

// TestMetricsAndDashboardDisabled starts only the monitoring server, no
// NATS/Kafka needed, and checks the disable flags remove the endpoints.
func TestMetricsAndDashboardDisabled(t *testing.T) {
	bridge := NewNATSKafkaBridge()
	bridge.config = conf.DefaultBridgeConfig()
	bridge.config.Monitoring = conf.HTTPConfig{
		HTTPPort:         -1,
		DisableMetrics:   true,
		DisableDashboard: true,
	}

	bridge.Lock()
	err := bridge.startMonitoring()
	bridge.Unlock()
	require.NoError(t, err)
	defer func() {
		bridge.Lock()
		defer bridge.Unlock()
		require.NoError(t, bridge.StopMonitoring())
	}()

	client := http.Client{}
	for _, path := range []string{"metrics", "dashboard"} {
		response, err := client.Get(bridge.GetMonitoringRootURL() + path)
		require.NoError(t, err)
		response.Body.Close()
		require.Equal(t, http.StatusNotFound, response.StatusCode)
	}

	response, err := client.Get(bridge.GetMonitoringRootURL() + "healthz")
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
}
