# Demo of NATS and Kafka

Docker is required.

To start the cluster with Kafka, ZooKeeper, NATS and NATS Streaming

`docker-compose up -d`

Docker Compose will create a default network nats-kafka_default
`docker-compose ps` will show you all the services.

## NATS

NATS core and streaming are availble. The NATS server is at nats:4222 when inside Docker environment.

## Kafka

We create 'foo', 'bar', and 'test' topics by default for Kafka.

## NATS to Kafka Bridge

This will be started and by default be running a mapping as follows. This can be overridden.
Note that the mapping to and from NATS does both NATS and STAN

```
Mapping Inbound NATS Subject "foo" to Kafka Topic: "foo"
Mapping Inbound Kafka Topic: "bar" to NATS Subject: "bar"
```

Start a kafka shell inside docker

`./kafka-shell.sh`

```bash
$KAFKA_HOME/bin/kafka-console-consumer.sh --topic=foo --bootstrap-server=kafka:9092
```

Start another shell in new terminal window

`./kafka-shell.sh`

```bash
$KAFKA_HOME/bin/kafka-console-producer.sh --topic=bar --broker-list=kafka:9092 --sync --timeout=0
```

Note: Kafka Producer is interactive, enter some text hit <return> to send.

Also start up a NATS and STAN subscriber. You do not have to do these inside the docker env.

```
stan-sub -c STAN -id=test bar
nats-sub bar
```

To start a shell with all the nats-tools inside docker

`./nats-tools.sh` pops you into a container that has all the tools available.

```bash
# nats-sub -s nats bar &
# stan-sub -s nats -c STAN -id test --last bar &

# nats-pub -s nats foo hello
# stan-pub -s nats -c STAN foo hello

```
