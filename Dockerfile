FROM golang:1.11-alpine

MAINTAINER Derek Collison <derek@nats.io>

RUN apk add --no-cache git
RUN go get github.com/nats-io/go-nats/examples/nats-pub \
  && go get github.com/nats-io/go-nats/examples/nats-sub \
  && go get github.com/nats-io/go-nats/examples/nats-qsub \
  && go get github.com/nats-io/go-nats-streaming/examples/stan-sub \
  && go get github.com/nats-io/go-nats-streaming/examples/stan-pub
