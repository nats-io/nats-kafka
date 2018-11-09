#!/bin/bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --network nats-kafka_default -ti nats-tools /bin/sh
