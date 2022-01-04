![NATS](logos/large-logo.png)

# NATS-Kafka Bridge

[![License][License-Image]][License-Url]
[![ReportCard][ReportCard-Image]][ReportCard-Url]
[![Build][Build-Status-Image]][Build-Status-Url]
[![Coverage][Coverage-Image]][Coverage-Url]

This project implements a multi-connector bridge between NATS, NATS Streaming,
JetStream and Kafka topics.

## Features

* Support for bridging from/to Kafka topics
* Arbitrary subjects in NATS and JetStream, wildcards for incoming messages
* Arbitrary channels in NATS Streaming
* Optional durable subscriber names for streaming
* Configurable std-out logging
* A single configuration file, with support for reload
* Optional SSL to/from Kafka, NATS, NATS Streaming, and JetStream
* HTTP/HTTPS-based monitoring endpoints for health or statistics
* Support for Confluent schema registry and compatible implementations with Avro, JSON Schema and Protobuf.

## Overview

The bridge runs as a single process with a configured set of connectors mapping
a Kafka topic to a NATS/JetStream subject or a NATS Streaming channel.
Connectors can also map the opposite direction from NATS to Kafka. Each
connector is a one-way bridge.

Connectors share a NATS connection and an optional connection to the NATS
Streaming server. **Connectors each create a connection to Kafka, subject to
TCP connection sharing in the underlying library**

Message values in Kafka are mapped to message bodies in NATS.

Messages coming from NATS to Kafka can have their key set in a variety of ways,
see [Configuration](docs/config.md). Messages coming from Kafka will have their
key ignored.

Request-reply is not supported.

The bridge is [configured with a NATS server-like format](docs/config.md), in a
single file and uses the NATS logger.

An [optional HTTP/HTTPS endpoint](docs/monitoring.md) can be used for
monitoring.

## Documentation

* [Build & Run the Bridge](docs/buildandrun.md)
* [Configuration](docs/config.md)
* [Monitoring](docs/monitoring.md)

## External Resources

* [NATS](https://nats.io/documentation/)
* [NATS Server](https://github.com/nats-io/nats-server)
* [NATS Streaming](https://github.com/nats-io/nats-streaming-server)
* [JetStream](https://docs.nats.io/jetstream/jetstream)
* [Kafka](https://kafka.apache.org/)
* [Confluent Schema Registry](https://docs.confluent.io/platform/current/schema-registry/index.html)

[License-Url]: https://www.apache.org/licenses/LICENSE-2.0
[License-Image]: https://img.shields.io/badge/License-Apache2-blue.svg
[Build-Status-Url]: https://github.com/nats-io/nats-kafka/actions/workflows/testing.yaml
[Build-Status-Image]: https://github.com/nats-io/nats-kafka/actions/workflows/testing.yaml/badge.svg?branch=main
[Coverage-Url]: https://app.codecov.io/gh/nats-io/nats-kafka
[Coverage-image]: https://codecov.io/gh/nats-io/nats-kafka/branch/main/graph/badge.svg
[ReportCard-Url]: https://goreportcard.com/report/nats-io/nats-kafka
[ReportCard-Image]: https://goreportcard.com/badge/github.com/nats-io/nats-kafka

<a name="license"></a>

## License

Unless otherwise noted, the nats-kafka bridge source files are distributed
under the Apache Version 2.0 license found in the LICENSE file.
