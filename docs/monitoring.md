# Monitoring the NATS-Kafka Bridge

The nats-kafka bridge provides optional HTTP/s monitoring. When [configured with a monitoring port](config.md#monitoring) the server will provide four HTTP endpoints:

* [/varz](#varz)
* [/healthz](#healthz)
* [/metrics](#metrics)
* [/dashboard](#dashboard)

<a name="varz"></a>

## /varz

The `/varz` endpoint returns a JSON encoded set of statistics for the server. These statistics are wrapped in a root level object with the following properties:

* `start_time` - the start time of the bridge, in the bridge's timezone.
* `current_time` - the current time, in the bridge's timezone.
* `uptime` - a string representation of the server's up time.
* `http_requests` - a map of request paths to counts, the keys are `/`, `/varz`, `/healthz`, `/metrics` and `/dashboard`.
* `nats` - an array with the connection state and wire-level statistics of each NATS cluster (cell), sorted by name.
* `connectors` - an array of statistics for each connector.

Each object in the nats array, one per configured cluster, will contain the following properties:

* `name` - the cluster name, `default` for the `nats` config block.
* `connected` - true when the bridge currently has an established connection to the cluster.
* `connected_url` - the server URL the bridge is connected to, omitted while disconnected.
* `reconnects` - the number of reconnects on this cluster's connection.
* `in_msgs`, `out_msgs` - messages received from and sent to this cluster, as reported by the NATS client.
* `in_bytes`, `out_bytes` - bytes received from and sent to this cluster, as reported by the NATS client. STAN traffic flows over the default cluster's connection and is included in its numbers.

Each object in the connectors array, one per connector, will contain the following properties:

* `name` - the name of the connector, a human readable description of the connector.
Connectors bound to a named NATS cluster are prefixed with the cluster name in brackets.
* `id` - the connectors id, either set in the configuration or generated at runtime.
* `nats_connection` - the named NATS cluster the connector is bound to, omitted for the default cluster.
* `connects` - a count of the number of times the connector has connected.
* `disconnects` -  a count of the number of times the connector has disconnected.
* `bytes_in` - the number of bytes the connector has received, may differ from received due to headers and encoding.
* `bytes_out` - the number of bytes the connector has sent, may differ from received due to headers and encoding.
* `msg_in` - the number of messages received.
* `msg_out` - the number of messages sent.
* `count` - the total number of requests for this connector.
* `rma` - a [running moving average](https://en.wikipedia.org/wiki/Moving_average) of the time required to handle each request. The time is in nanoseconds.
* `q50` - the 50% quantile for response times, in nanoseconds.
* `q75` - the 75% quantile for response times, in nanoseconds.
* `q90` - the 90% quantile for response times, in nanoseconds.
* `q95` - the 95% quantile for response times, in nanoseconds.
* `num_pending` - for JetStream sources, the consumer backlog observed on the last delivered message.

The `nats` object contains the statistics reported by the NATS client library for the shared connection.
STAN traffic flows over the same connection and is included in these numbers.

* `connected` - whether the connection is currently up.
* `connected_url` - the URL of the NATS server the bridge is connected to.
* `reconnects` - the number of reconnects.
* `in_msgs` / `out_msgs` - messages received from / sent to NATS.
* `in_bytes` / `out_bytes` - bytes received from / sent to NATS.

<a name="healthz"></a>

## /healthz

The `/healthz` endpoint is provided for automated up/down style checks. The server returns an HTTP/200 when running and won't respond if it is down.

<a name="metrics"></a>

## /metrics

The `/metrics` endpoint exposes the same statistics in the Prometheus exposition format, so a Prometheus server can scrape the bridge directly.
It is served whenever monitoring is enabled and can be turned off with the `disablemetrics` [configuration setting](config.md#monitoring).
Standard Go runtime and process metrics are included alongside the bridge metrics below.

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `nats_kafka_bridge_info` | gauge | `version` | Build information, value is always 1. |
| `nats_kafka_bridge_start_time_seconds` | gauge | | Unix time the bridge started. |
| `nats_kafka_bridge_http_requests_total` | counter | `path` | Requests served by the monitoring endpoints. |
| `nats_kafka_bridge_connector_connected` | gauge | `connector` | Whether the connector is connected (1) or not (0). |
| `nats_kafka_bridge_connector_connects_total` | counter | `connector` | Times the connector has connected. |
| `nats_kafka_bridge_connector_disconnects_total` | counter | `connector` | Times the connector has disconnected. |
| `nats_kafka_bridge_connector_messages_in_total` | counter | `connector` | Messages read from the connector's source. |
| `nats_kafka_bridge_connector_messages_out_total` | counter | `connector` | Messages successfully written to the destination. |
| `nats_kafka_bridge_connector_bytes_in_total` | counter | `connector` | Payload bytes read from the source. |
| `nats_kafka_bridge_connector_bytes_out_total` | counter | `connector` | Payload bytes successfully written to the destination. |
| `nats_kafka_bridge_connector_requests_total` | counter | `connector` | Messages fully processed by the connector. |
| `nats_kafka_bridge_connector_request_duration_seconds` | summary | `connector` | End-to-end handling time, quantiles 0.5/0.75/0.9/0.95. |
| `nats_kafka_bridge_connector_pending_messages` | gauge | `connector` | JetStream consumer backlog at the last delivered message. |
| `nats_kafka_bridge_nats_connected` | gauge | | Whether the shared NATS connection is up. |
| `nats_kafka_bridge_nats_reconnects_total` | counter | | Reconnects on the shared NATS connection. |
| `nats_kafka_bridge_nats_in_msgs_total` | counter | | Messages received over the shared NATS connection. |
| `nats_kafka_bridge_nats_out_msgs_total` | counter | | Messages sent over the shared NATS connection. |
| `nats_kafka_bridge_nats_in_bytes_total` | counter | | Bytes received over the shared NATS connection. |
| `nats_kafka_bridge_nats_out_bytes_total` | counter | | Bytes sent over the shared NATS connection. |

Notes:

* The `connector` label is the connector's configured name, which stays stable across restarts.
* When a failing connector is restarted by the bridge its counters reset to zero. Prometheus `rate()` and `increase()` handle these resets natively.
* The duration summary quantiles are computed over the connector's lifetime, not a sliding window, and cannot be aggregated across bridge instances.

A minimal scrape config:

```yaml
scrape_configs:
  - job_name: nats-kafka-bridge
    static_configs:
      - targets: ["bridge-host:9222"]
```

<a name="dashboard"></a>

## /dashboard

The `/dashboard` endpoint serves a self-contained browser dashboard with no external dependencies.
It polls `/varz` every few seconds and shows NATS wire traffic rates (bytes read from and written to NATS), aggregate and per-connector message rates, cumulative totals, failed deliveries, JetStream backlog, and message handling latency quantiles.
It is served whenever monitoring is enabled and can be turned off with the `disabledashboard` [configuration setting](config.md#monitoring).
