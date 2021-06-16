/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/kafka"
	"github.com/nats-io/nuid"
)

var iterations int
var topics int
var kafkaHostPort string
var connectTimeout int

func main() {
	flag.IntVar(&connectTimeout, "t", 10000, "connection timeout")
	flag.IntVar(&iterations, "i", 100, "iterations, defaults to 100")
	flag.IntVar(&topics, "t", 1, "number of simultaneous topics to use, defaults to 1")
	flag.StringVar(&kafkaHostPort, "kafka", "localhost:9092", "kafka host:port")
	flag.Parse()

	msgString := strings.Repeat("stannats", 128) // 1024 bytes
	msg := []byte(msgString)
	msgLen := len(msg)

	interval := int(iterations / 10)

	ready := sync.WaitGroup{}
	done := sync.WaitGroup{}
	starter := sync.WaitGroup{}

	starter.Add(1)
	ready.Add(topics * 2)
	done.Add(topics)

	for i := 0; i < topics; i++ {
		topic := nuid.Next()
		log.Printf("creating topic %s", topic)
		connection, err := kafka.NewManager(conf.ConnectorConfig{
			Brokers: []string{kafkaHostPort},
		}, conf.NATSKafkaBridgeConfig{
			ConnectTimeout: connectTimeout,
		})
		if err != nil {
			log.Fatalf("unable to connect to kafka server")
		}

		err = connection.CreateTopic(topic, 1, 1)
		connection.Close()
		if err != nil {
			log.Fatalf("error creating topic, %s", err.Error())
		}

		go startWriter(topic, msg, interval, &ready, &starter)
		go startReader(topic, msgLen, interval, &ready, &starter, &done)
	}

	ready.Wait()
	start := time.Now()
	starter.Done()
	done.Wait()
	end := time.Now()

	count := iterations * topics
	diff := end.Sub(start)
	rate := float64(count) / float64(diff.Seconds())
	log.Printf("Sent %d messages through %d topics in %s, or %.2f msgs/sec", count, topics, diff, rate)
}

func startWriter(topic string, msg []byte, interval int, ready, starter *sync.WaitGroup) {
	writer, err := kafka.NewProducer(conf.ConnectorConfig{
		Brokers: []string{kafkaHostPort},
		Topic:   topic,
	}, conf.NATSKafkaBridgeConfig{
		ConnectTimeout: connectTimeout,
	}, topic)
	if err != nil {
		log.Fatalf("unable to connect to kafka server")
	}

	log.Printf("sender ready for topic %s", topic)
	ready.Done()
	starter.Wait()
	log.Printf("sending %d messages through %s kafka...", iterations, topic)
	for i := 0; i < iterations; i++ {
		err := writer.Write(kafka.Message{
			Key:   []byte(topic),
			Value: msg,
		})
		if err != nil {
			log.Fatalf("error putting messages on topic, %s", err.Error())
		}
		if i%interval == 0 && i != 0 {
			log.Printf("%s: send count = %d", topic, i)
		}
	}

	writer.Close()
}

func startReader(topic string, msgLen, interval int, ready, starter, done *sync.WaitGroup) {
	reader, err := kafka.NewConsumer(conf.ConnectorConfig{
		Brokers:  []string{kafkaHostPort},
		Topic:    topic,
		GroupID:  topic + ".grp",
		MinBytes: 100,
		MaxBytes: 10e6, // 10MB
	}, 5*time.Second)
	if err != nil {
		log.Fatalf("unable to connect to kafka server")
	}

	log.Printf("receiver ready for topic %s", topic)
	ready.Done()
	starter.Wait()
	log.Printf("reading %d messages from %s kafka...", iterations, topic)

	count := 0
	for {
		m, err := reader.Fetch(context.Background())
		if err != nil {
			log.Fatalf("read error on %s, %s", topic, err.Error())
		}

		err = reader.Commit(context.Background(), m)
		if err != nil {
			log.Fatalf("commit error on %s, %s", topic, err.Error())
		}

		if len(m.Value) != msgLen {
			log.Fatalf("bad message length %s, %d != %d", topic, len(m.Value), msgLen)
		}

		count++
		if count%interval == 0 {
			log.Printf("%s: receive count = %d", topic, count)
		}
		if count == iterations {
			done.Done()
		}
	}
}
