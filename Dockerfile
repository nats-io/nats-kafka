FROM golang:1.12.4 AS builder

WORKDIR /src/nats-kafka

LABEL maintainer "Stephen Asbury <sasbury@nats.io>"

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -v -a -tags netgo -installsuffix netgo -o /nats-kafka

FROM alpine:3.9

RUN mkdir -p /nats/bin && mkdir /nats/conf

COPY --from=builder /nats-kafka /nats/bin/nats-kafka

RUN ln -ns /nats/bin/nats-kafka /bin/nats-kafka

ENTRYPOINT ["/bin/nats-kafka"]
