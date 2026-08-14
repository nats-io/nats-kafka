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
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsNamespace = "nats_kafka_bridge"

// Per-connector metrics are labeled with the connector's configured name,
// which is stable across restarts, unlike the generated nuid ID.
var (
	descInfo = prometheus.NewDesc(
		metricsNamespace+"_info",
		"Build information for the NATS-Kafka bridge, value is always 1.",
		[]string{"version"}, nil)
	descStartTime = prometheus.NewDesc(
		metricsNamespace+"_start_time_seconds",
		"Unix time the bridge started.",
		nil, nil)
	descHTTPRequests = prometheus.NewDesc(
		metricsNamespace+"_http_requests_total",
		"Requests served by the monitoring endpoints.",
		[]string{"path"}, nil)

	descConnConnected = prometheus.NewDesc(
		metricsNamespace+"_connector_connected",
		"Whether the connector is currently connected (1) or not (0).",
		[]string{"connector"}, nil)
	descConnConnects = prometheus.NewDesc(
		metricsNamespace+"_connector_connects_total",
		"Times the connector has connected.",
		[]string{"connector"}, nil)
	descConnDisconnects = prometheus.NewDesc(
		metricsNamespace+"_connector_disconnects_total",
		"Times the connector has disconnected.",
		[]string{"connector"}, nil)
	descConnMsgsIn = prometheus.NewDesc(
		metricsNamespace+"_connector_messages_in_total",
		"Messages read from the connector's source.",
		[]string{"connector"}, nil)
	descConnMsgsOut = prometheus.NewDesc(
		metricsNamespace+"_connector_messages_out_total",
		"Messages successfully written to the connector's destination.",
		[]string{"connector"}, nil)
	descConnBytesIn = prometheus.NewDesc(
		metricsNamespace+"_connector_bytes_in_total",
		"Payload bytes read from the connector's source.",
		[]string{"connector"}, nil)
	descConnBytesOut = prometheus.NewDesc(
		metricsNamespace+"_connector_bytes_out_total",
		"Payload bytes successfully written to the connector's destination.",
		[]string{"connector"}, nil)
	descConnRequests = prometheus.NewDesc(
		metricsNamespace+"_connector_requests_total",
		"Messages fully processed by the connector.",
		[]string{"connector"}, nil)
	descConnDuration = prometheus.NewDesc(
		metricsNamespace+"_connector_request_duration_seconds",
		"End-to-end message handling time. Quantiles are computed over the connector's lifetime, not a sliding window.",
		[]string{"connector"}, nil)
	descConnPending = prometheus.NewDesc(
		metricsNamespace+"_connector_pending_messages",
		"JetStream consumer backlog observed on the last delivered message, 0 for non-JetStream connectors.",
		[]string{"connector"}, nil)

	descNATSConnected = prometheus.NewDesc(
		metricsNamespace+"_nats_connected",
		"Whether the cluster's NATS connection is up (1) or not (0).",
		[]string{"cluster"}, nil)
	descNATSReconnects = prometheus.NewDesc(
		metricsNamespace+"_nats_reconnects_total",
		"Reconnects on the cluster's NATS connection.",
		[]string{"cluster"}, nil)
	descNATSInMsgs = prometheus.NewDesc(
		metricsNamespace+"_nats_in_msgs_total",
		"Messages received over the cluster's NATS connection.",
		[]string{"cluster"}, nil)
	descNATSOutMsgs = prometheus.NewDesc(
		metricsNamespace+"_nats_out_msgs_total",
		"Messages sent over the cluster's NATS connection.",
		[]string{"cluster"}, nil)
	descNATSInBytes = prometheus.NewDesc(
		metricsNamespace+"_nats_in_bytes_total",
		"Bytes received over the cluster's NATS connection.",
		[]string{"cluster"}, nil)
	descNATSOutBytes = prometheus.NewDesc(
		metricsNamespace+"_nats_out_bytes_total",
		"Bytes sent over the cluster's NATS connection.",
		[]string{"cluster"}, nil)
)

// bridgeCollector adapts the bridge's existing stats to the Prometheus
// exposition format, reading them fresh on every scrape.
type bridgeCollector struct {
	bridge *NATSKafkaBridge
}

// Describe implements prometheus.Collector. It lists the static descriptors
// rather than using DescribeByCollect, because registration happens while the
// bridge lock is held and a Collect there would deadlock on SafeStats.
func (c *bridgeCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		descInfo, descStartTime, descHTTPRequests,
		descConnConnected, descConnConnects, descConnDisconnects,
		descConnMsgsIn, descConnMsgsOut, descConnBytesIn, descConnBytesOut,
		descConnRequests, descConnDuration, descConnPending,
		descNATSConnected, descNATSReconnects,
		descNATSInMsgs, descNATSOutMsgs, descNATSInBytes, descNATSOutBytes,
	} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *bridgeCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.bridge.SafeStats()

	ch <- prometheus.MustNewConstMetric(descInfo, prometheus.GaugeValue, 1, Version)
	ch <- prometheus.MustNewConstMetric(descStartTime, prometheus.GaugeValue, float64(stats.StartTime))

	for path, count := range stats.HTTPRequests {
		ch <- prometheus.MustNewConstMetric(descHTTPRequests, prometheus.CounterValue, float64(count), path)
	}

	for _, cs := range stats.Connections {
		connected := 0.0
		if cs.Connected {
			connected = 1.0
		}
		ch <- prometheus.MustNewConstMetric(descConnConnected, prometheus.GaugeValue, connected, cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnConnects, prometheus.CounterValue, float64(cs.Connects), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnDisconnects, prometheus.CounterValue, float64(cs.Disconnects), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnMsgsIn, prometheus.CounterValue, float64(cs.MessagesIn), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnMsgsOut, prometheus.CounterValue, float64(cs.MessagesOut), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnBytesIn, prometheus.CounterValue, float64(cs.BytesIn), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnBytesOut, prometheus.CounterValue, float64(cs.BytesOut), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnRequests, prometheus.CounterValue, float64(cs.RequestCount), cs.Name)
		ch <- prometheus.MustNewConstMetric(descConnPending, prometheus.GaugeValue, float64(cs.NumPending), cs.Name)

		// Quantiles come back as -1 when the histogram is empty.
		quantiles := map[float64]float64{}
		if cs.RequestCount > 0 {
			quantiles[0.5] = cs.Quintile50 / float64(nanosPerSecond)
			quantiles[0.75] = cs.Quintile75 / float64(nanosPerSecond)
			quantiles[0.9] = cs.Quintile90 / float64(nanosPerSecond)
			quantiles[0.95] = cs.Quintile95 / float64(nanosPerSecond)
		}
		sumSeconds := cs.MovingAverage * float64(cs.RequestCount) / float64(nanosPerSecond)
		ch <- prometheus.MustNewConstSummary(descConnDuration,
			uint64(cs.RequestCount), sumSeconds, quantiles, cs.Name)
	}

	for _, ns := range stats.NATS {
		connected := 0.0
		if ns.Connected {
			connected = 1.0
		}
		ch <- prometheus.MustNewConstMetric(descNATSConnected, prometheus.GaugeValue, connected, ns.Name)
		ch <- prometheus.MustNewConstMetric(descNATSReconnects, prometheus.CounterValue, float64(ns.Reconnects), ns.Name)
		ch <- prometheus.MustNewConstMetric(descNATSInMsgs, prometheus.CounterValue, float64(ns.InMsgs), ns.Name)
		ch <- prometheus.MustNewConstMetric(descNATSOutMsgs, prometheus.CounterValue, float64(ns.OutMsgs), ns.Name)
		ch <- prometheus.MustNewConstMetric(descNATSInBytes, prometheus.CounterValue, float64(ns.InBytes), ns.Name)
		ch <- prometheus.MustNewConstMetric(descNATSOutBytes, prometheus.CounterValue, float64(ns.OutBytes), ns.Name)
	}
}

const nanosPerSecond = int64(1e9)

// newMetricsHandler builds the /metrics handler on a private registry so
// multiple bridges in one process never collide on metric registration.
func newMetricsHandler(bridge *NATSKafkaBridge) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		&bridgeCollector{bridge: bridge},
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	promHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bridge.statsLock.Lock()
		bridge.httpReqStats[MetricsPath]++
		bridge.statsLock.Unlock()
		promHandler.ServeHTTP(w, r)
	})
}
