FROM golang:1.11-alpine

MAINTAINER Derek Collison <derek@nats.io>

COPY . /go/src/github.com/nats-io/nats-kafka/nkb
WORKDIR /go/src/github.com/nats-io/nats-kafka/nkb

RUN apk add --no-cache git

RUN go get github.com/nats-io/nats-kafka/nkb

ENTRYPOINT ["nkb"]
