# NATS-Kafka Bridge Configuration

The bridge uses a single configuration file passed on the command line or environment variable. Configuration is organized into a root section and several blocks.

* [Specifying the Configuration File](#specify)
* [Shared](#root)
* [TLS](#tls)
* [SASL](#sasl)
* [Logging](#logging)
* [Monitoring](#monitoring)
* [NATS](#nats)
* [NATS Streaming](#stan)
* [JetStream](#js)
* [Connectors](#connectors)

The configuration file format matches the NATS server. Details can be found [here](https://docs.nats.io/nats-server/configuration#include-directive).

**IMPORTANT**: Includes *must* use relative paths, and are relative to the main configuration.

```yaml
include "./includes/connectors.conf"
```

<a name="specify"></a>

## Specifying the Configuration File

To set the configuration on the command line, use:

```bash
% nats-kafka -c <path to config file>
```

To set the configuration file using an environment variable, export `NATS_KAFKA_BRIDGE_CONFIG` with the path to the configuration.

<a name="root"></a>

## Root Section

The root section:

```yaml
reconnectinterval: 5000,
connecttimeout: 5000,
```

can currently contain settings for:

* `reconnectinterval` - this value, in milliseconds, is the time used in between reconnection attempts for a connector when it fails. For example, if a connector loses access to NATS, the bridge will try to restart it every `reconnectinterval` milliseconds.
* `connecttimeout` - this value, in milliseconds, is the time used when trying to connect to Kafka.

## TLS <a name="tls"></a>

NATS, streaming, Kafka and HTTP configurations all take an optional TLS setting. The TLS configuration takes three possible settings:

* `root` - file path to a CA root certificate store, used for NATS connections
* `cert` - file path to a server certificate, used for HTTPS monitoring and optionally for client side certificates with NATS
* `key` - key for the certificate store specified in cert

## SASL <a name="sasl"></a>

Kafka allow for SASL_PLAIN connection with user and password.  This also supports a flag to connect to Azure EventHub

For each connector section as needed:

* `insecureskipverify` - (optional) allow for auto-adjustment for TLS handshake default to false (Azure EventHub requires this to be `true`)
```yaml
"sasl": {
  "user": "userid",
  "password": "password"
}
```
<a name="logging"></a>

### Logging

Logging is configured in a manner similar to the nats-server:

```yaml
logging: {
  time: true,
  debug: false,
  trace: false,
  colors: true,
  pid: false,
}
```

These properties are configured for:

* `time` - include the time in logging statements
* `debug` - include debug logging
* `trace` - include verbose, or trace, logging
* `colors` - colorize the logging statements
* `pid` - include the process id in logging statements

<a name="monitoring"></a>

## Monitoring

The monitoring section:

```yaml
monitoring: {
  httpsport: -1,
  tls: {
      cert: /a/server-cert.pem,
      key: /a/server-key.pem,
  }
}
```

Is used to configure an HTTP or HTTPS port, as well as TLS settings when HTTPS is used.

* `httphost` - the network interface to publish monitoring on, valid for HTTP or HTTPS. An empty value will tell the server to use all available network interfaces.
* `httpport` - the port for HTTP monitoring, no TLS configuration is expected, a value of -1 will tell the server to use an ephemeral port, the port will be logged on startup.

`2019/03/20 12:06:38.027822 [INF] starting http monitor on :59744`

* `httpsport` - the port for HTTPS monitoring, a TLS configuration is expected, a value of -1 will tell the server to use an ephemeral port, the port will be logged on startup.
* `tls` - a [TLS configuration](#tls).

The `httpport` and `httpsport` settings are mutually exclusive, if both are set to a non-zero value the bridge will not start.

<a name="nats"></a>

## NATS

The bridge makes a single connection to NATS. This connection is shared by all connectors. Configuration is through the `nats` section of the config file:

```yaml
nats: {
  Servers: ["localhost:4222"],
  ConnectTimeout: 5000,
  MaxReconnects: 5,
  ReconnectWait: 5000,
}
```

NATS can be configured with the following properties:

* `servers` - an array of server URLS
* `connecttimeout` - the time, in milliseconds, to wait before failing to connect to the NATS server
* `reconnectwait` - the time, in milliseconds, to wait between reconnect attempts
* `maxreconnects` - the maximum number of reconnects to try before exiting the bridge with an error.
* `tls` - (optional) [TLS configuration](#tls). If the NATS server uses unverified TLS with a valid certificate, this setting isn't required.
* `usercredentials` - (optional) the path to a credentials file for connecting to NATs.

<a name="stan"></a>

## NATS Streaming

The bridge makes a single connection to a NATS streaming server. This connection is shared by all connectors. Configuration is through the `stan` section of the config file:

```yaml
stan: {
  ClusterID: "test-cluster"
  ClientID: "kafkabridge"
}
```

NATS streaming can be configured with the following properties:

* `clusterid` - the cluster id for the NATS streaming server.
* `clientid` - the client id for the bridge, shared by all connections.
* `pubackwait` - the time, in milliseconds, to wait before a publish fails due to a timeout.
* `discoverprefix` - the discover prefix for the streaming server.
* `maxpubacksinflight` - maximum pub ACK messages that can be in flight for this connection.
* `connectwait` - the time, in milliseconds, to wait before failing to connect to the streaming server.

<a name="js"></a>

## JetStream

The bridge makes a single connection to JetStream. This connection is shared by all connectors. Configuration is through the `jetstream` section of the config file:

```
jetstream: {
	maxwait: 5000,
}
```

JetStream can be configured with the following properties:

* `publishasyncmaxpending` - maximum outstanding async publishes that can be inflight at one time
* `maxwait` - maximum amount of time we will wait for a response
* `enableflowcontrol` - enables flow control for a push based consumer
* `enableacksync` - uses AckSync instead of Ack when receiving messages
* `heartbeatinterval` - enables push based consumers to have idle heartbeats delivered

<a name="connectors"></a>

## Connectors

The final piece of the bridge configuration is the `connect` section. Connect specifies an array of connector configurations. All connector configs use the same format, relying on optional settings to determine what the do.

```yaml
connect: [
  {
      type: "KafkaToNATS",
      brokers: ["localhost:9092"]
      id: "zip",
      topic: "test",
      subject: "one",
  },{
      type: "NATSToKafka",
      brokers: ["localhost:9092"]
      id: "zap",
      topic: "test2",
      subject: "two",
  },
],
```

The most important property in the connector configuration is the `type`. The type determines which kind of connector is instantiated. Available, uni-directional, types include:

* `KafkaToNATS` - a topic to NATS connector
* `KafkaToStan` - a topic to NATS Streaming connector
* `KafkaToJetStream` - a topic to JetStream connector

* `NATSToKafka` - a NATS to topic connector
* `STANToKafka` - a NATS Streaming to topic connector
* `JetStreamToKafka` - a JetStream to topic connector

All connectors can have an optional id, which is used in monitoring:

* `id` - (optional) user defined id that will tag the connection in monitoring JSON.

For NATS connections, specify:

* `subject` - for NATS/JetStream the subject to subscribe/publish to, depending on the connections direction.
* `queuename` - the queue group to use in subscriptions, this is optional but useful for load balancing.

Keep in mind that NATS queue groups do not guarantee ordering, since the queue subscribers can be on different nats-servers in a cluster. So if you have to bridges running with connectors on the same NATS queue/subject pair and have a high message rate you may get messages in the Kafka topic out of order.

For NATS Streaming connections, there is a single required setting and several optional ones:

* `channel` - the NATS Streaming channel to subscribe/publish to.
* `durablename` - (optional) durable name for the NATS Streaming/JetStream subscription (if appropriate.)
* `startatsequence` - (optional) for NATS Streaming/JetStream start position, use -1 for start with last received, 0 for deliver all available (the default.)
* `startattime` - (optional) for NATS Streaming/JetStream the start position as a time, in Unix seconds since the epoch, mutually exclusive with `startatsequence`.

All connectors must specify Kafka connection properties, with a few optional settings available as well:

* `brokers` - a string array of broker host:port settings
* `topic` - the Kafka topic to listen/send to
* `tls` - A tls config for the connection
* `sasl` - the Kafka userid and password for connection
* `balancer` - required for a writer, should be "hash" or "leastbytes"
* `groupid` - (exclusive with partition) used by the reader to set a group id
* `partition` - (exclusive with groupid) used by the reader to set a partition
* `minbytes` - (optional) used by Kafka readers to set the minimum bytes for a read
* `maxbytes` - (optional) used by a Kafka reader to set the maximum bytes for a read
* `keytype` - (optional) defines the way keys are assigned to messages coming from NATS (see below)
* `keyvalue` - (optional) extra data that may be used depending on the key type
* `schemaregistryurl` - (optional) URL of the Kafka schema registry instance
* `subjectname` - (exclusive with schemaregistryurl) Name of the subject in the schema registry to use for schema
* `schemaversion` - (optional, exclusive with schemaregistryurl) Version of the schema to use from the registry, uses the latest if unspecified
* `schematype` - (optional, exclusive with schemaregistryurl) Type of schema. Can be "avro", "json" or "protobuf", default is "avro"

Available key types are:

* `fixed` - the value in the `keyvalue` field is assigned to all messages
* `subject` - the subject the incoming NATS message was sent on is used as the key
* `reply` - the reply-to sent with the incoming NATS messages is used as the key
* `subjectre` - the value in the `keyvalue` field used as a regular expression and the first capture group, when matched to the subject, is used as the key
* `replyre` - the value in the `keyvalue` field used as a regular expression and the first capture group, when matched to the reply-to, is used as the key

If unset, an empty key is assigned during translation from NATS to Kafka. If the regex types are used and they don't match, an empty key is used.

For NATS Streaming connections channel is treated as the subject and durable name is treated as the reply to, so that reply key type will use the durable name as the key.

Every destination, may it be channel, subject or topic may be set using a go template that gets the message as data. e.g `topic: "{{ .Subject }}"`. Two template functions are available: replace (`"{{ .Subject | replace \"event\" \"message\" }}`) and substring (`"{{ .Subject | substring 0 5 }}"`)
