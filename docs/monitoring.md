# Monitoring the NATS-Kafka Bridge

The nats-kafka bridge provides optional HTTP/s monitoring. When [configured with a monitoring port](config.md#monitoring) the server will provide two HTTP endpoints:

* [/varz](#varz)
* [/healthz](#healthz)

<a name="varz"></a>

## /varz

The `/varz` endpoint returns a JSON encoded set of statistics for the server. These statistics are wrapped in a root level object with the following properties:

* `start_time` - the start time of the bridge, in the bridge's timezone.
* `current_time` - the current time, in the bridge's timezone.
* `uptime` - a string representation of the server's up time.
* `http_requests` - a map of request paths to counts, the keys are `/`, `/varz` and `/healthz`.
* `connectors` - an array of statistics for each connector.

Each object in the connectors array, one per connector, will contain the following properties:

* `name` - the name of the connector, a human readable description of the connector.
* `id` - the connectors id, either set in the configuration or generated at runtime.
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

<a name="healthz"></a>

## /healthz

The `/healthz` endpoint is provided for automated up/down style checks. The server returns an HTTP/200 when running and won't respond if it is down.