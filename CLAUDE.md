# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the NATS-Kafka Bridge: a single Go binary that runs a configured set of one-way connectors between Kafka topics and NATS subjects, JetStream subjects, or NATS Streaming (STAN) channels.
Entry point is `main.go`, which parses flags and delegates to `server/core.NATSKafkaBridge`.

## Common Commands

```bash
make build            # go build -> ./nats-kafka binary
make lint             # gofmt -s, goimports, misspell, go vet, staticcheck
make install-tools    # install staticcheck, misspell, goimports (needed once for lint)
make test             # full test cycle: starts docker test servers, runs tests, tears down
make test-failfast    # same but with -failfast and without -short
make test-cover       # same but with coverage HTML report
```

Most tests are integration tests that need live Kafka, Zookeeper, and NATS servers.
Start and stop them manually when iterating:

```bash
make setup-docker-test     # docker compose -f resources/test_servers.yml up, waits for readiness
make run-test              # go test -count=1 -timeout 5m -short -race ./...
make teardown-docker-test
```

Run a single test (docker servers must already be up):

```bash
go test -count=1 -race -run TestSimpleSendOnNatsReceiveOnKafka ./server/core/...
```

Notes:

* CI (`.github/workflows/testing.yaml`) runs `make lint` and `make test-codecov` on Go 1.25.x.
* `-short` skips tests known to be flaky; `make test-failfast` runs the full set.
* Tests assume Kafka on `localhost:9092` (see `server/core/server_test.go` for the test bridge setup).

## Architecture

The bridge (`server/core/server.go`, `NATSKafkaBridge`) owns one NATS connection per configured cluster (cell), an optional STAN connection (default cluster only), and a JetStream context per cluster that needs one.
The default cluster comes from the `nats` config block; additional named clusters come from the optional `natsclusters` list, and connectors select one with `natsconnection` (see `server/core/nats.go`, `natsCell`).
Each connector creates its own Kafka connection.

**Connectors** are the central abstraction (`server/core/connector.go`, `Connector` interface: Start, Shutdown, CheckConnections, Stats).
There are six connector types, one file each in `server/core/`, created by the `CreateConnector` factory switch:
`nats2kafka`, `stan2kafka`, `jetstream2kafka`, `kafka2nats`, `kafka2stan`, `kafka2jetstream`.
All embed `BridgeConnector` (in `connector.go`), which holds shared logic such as NATS subscription setup, message key calculation (fixed, subject, reply-to, or regex-based), and stats/histogram tracking.

**Kafka layer** (`server/kafka/`) wraps Shopify/sarama and segmentio/kafka-go behind `Producer` and `Consumer` interfaces built via `manager.go`.
It also contains SASL/SCRAM and AWS IAM auth (`scram_client.go`, `iam_token_provider.go`), and Confluent Schema Registry support with Avro, JSON Schema, and Protobuf serialization (`pb_serializer.go`, `pb_deserializer.go`, `pb_schema_manager.go`).

**Configuration** (`server/conf/`) parses a NATS-server-style config file format (not YAML/JSON, though it is JSON-like).
The full config struct is `NATSKafkaBridgeConfig` in `server/conf/conf.go`; each connector is a `ConnectorConfig` where `Type` selects the connector kind.
Config file path comes from `-c` flag or `$NATS_KAFKA_BRIDGE_CONFIG`.
Example configs live in `conf/`; config documentation is in `docs/config.md`.

**Failure handling**: a failing connector reports via `NATSKafkaBridge.ConnectorError`, which shuts that connector down and adds it to a reconnect map; a timer (`reconnecttimer.go`) retries it every `reconnectinterval` milliseconds.

**Monitoring** (`server/core/monitoring.go`) exposes optional HTTP/HTTPS endpoints (`/varz`, `/healthz`) documented in `docs/monitoring.md`.

## Conventions

* Source files carry the Apache 2.0 license header; add it to new files.
* The module path is `github.com/nats-io/nats-kafka` even though this fork lives elsewhere; keep imports consistent with it.
