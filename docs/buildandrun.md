# Build and Run the NATS-Kafka Bridge

## Running the server

The server will compile to an executable named `nats-kafka`. A
[configuration](config.md) file is required to start the server.

To specify the [configuration](config.md) file, use the `-c` flag:

```bash
% nats-kafka -c <config file>
```

You can use the `-D`, `-V` or `-DV` flags to turn on debug or verbose logging.
The `-DV` option will turn on all logging, depending on the config file
settings, these settings will override the ones in the config file.

## Building the Server

This project uses go modules and provides a make file. You should be able to
simply:

```bash
% git clone https://github.com/nats-io/nats-kafka.git
% cd nats-kafka
% make build
```

Targets
* `make build` - build `nats-kafka`
* `make install` - build `nats-kafka` and move to your Go bin folder
* `make test` - setup Docker containers, run Go tests, teardown containers
* `make test-failfast` - like `make test`, but stop on first failure
* `make test-cover` - like `make test`, but get coverage report

## Docker

You can build the docker image using:

```bash
% make docker dtag=0.5
```

Then run it with:

```bash
% docker run -v <path to config>:/conf/kafkabridge.conf "natsio/nats-kafka:0.2.0" -c /conf/kafkabridge.conf
```

Be sure to include your monitoring port, for example, if port 9090 is used for monitoring, you can run with:

```bash
% docker run -v <path to config>:/conf/kafkabridge.conf -p 9090:9090 "natsio/nats-kafka:0.2.0" -c /conf/kafkabridge.conf
```
