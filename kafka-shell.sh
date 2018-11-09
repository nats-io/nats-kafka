#!/bin/bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --network nats-kafka_default -e HOST_IP=kafka -e ZK=zookeeper:2181 -i -t wurstmeister/kafka /bin/bash
