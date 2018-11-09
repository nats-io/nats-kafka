// Copyright 2018 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	"github.com/segmentio/kafka-go"
)

var usageStr = `
Usage: nkb [options]

Options:
	-ns,  <nats_url>      NATS Server URL
	-ks,  <kafka_url>     Kafka Broker URL
    -n2k, <subject:topic> NATS Subject to Kafka Topic
    -k2n, <topic:subject> Kafka Topic to NATS Subject
`

func usage() {
	log.Fatal(usageStr)
}

func main() {
	var (
		nats_url  = flag.String("ns", nats.DefaultURL, "The nats server URL")
		kafka_url = flag.String("ks", "localhost:9092", "The kafka broker URL")
		n2kMap    = flag.String("n2k", "foo:foo", "Map NATS Subject to Kafka Topic")
		k2nMap    = flag.String("k2n", "bar:bar", "Map Kafka Topic to NATS Subject")
	)

	// To hold our subjects and topics.
	nIn, nOut, kIn, kOut := mapSubjectsAndTopics(*n2kMap, *k2nMap)

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	opts := []nats.Option{nats.Name("NATS-Kafka Bridge")}
	opts = append(opts, nats.NoEcho())

	// Connect to NATS
	nc, err := nats.Connect(*nats_url, opts...)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to STAN
	sc, err := stan.Connect("STAN", "nkb", stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}

	// Create a Kafka Writer
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{*kafka_url},
		Topic:    kOut,
		Balancer: &kafka.LeastBytes{},
	})
	defer w.Close()

	key := []byte("NKB")

	// Do STAN inbound
	_, err = sc.Subscribe(nIn, func(m *stan.Msg) {
		w.WriteMessages(context.Background(),
			kafka.Message{
				Key:   key,
				Value: m.Data,
			},
		)
	})

	if err != nil {
		log.Fatal(err)
	}

	// Do NATS
	_, err = nc.Subscribe(nIn, func(m *nats.Msg) {
		w.WriteMessages(context.Background(),
			kafka.Message{
				Key:   key,
				Value: m.Data,
			},
		)
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create Readers and Writer for Kafka for the given subject.
	// FIXME(dlc) - Make these consumer groups for horizontal scaling.
	// Can use NATS qsubs as well.

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{*kafka_url},
		Topic:     kIn,
		Partition: 0,
		MinBytes:  8,    //10e3, // 10KB
		MaxBytes:  10e6, // 10MB
		//		CommitInterval: time.Second, // flushes commits to Kafka every second
	})
	defer r.Close()

	r.SetOffset(kafka.LastOffset)

	for {
		ctx := context.Background()
		m, err := r.FetchMessage(ctx)
		if err != nil {
			log.Fatal(err)
		}
		if err := nc.Publish(nOut, m.Value); err != nil {
			log.Print(err)
		}
		if err := sc.Publish(nOut, m.Value); err != nil {
			log.Print(err)
		}
		r.CommitMessages(ctx, m)
	}

}

func mapSubjectsAndTopics(n2k, k2n string) (nIn, nOut, kIn, kOut string) {
	if n2ks := strings.Split(n2k, ":"); len(n2ks) != 2 {
		log.Fatalf("Can't decode mapping for %q", n2k)
	} else {
		nIn, kOut = n2ks[0], n2ks[1]
	}
	if k2ns := strings.Split(k2n, ":"); len(k2ns) != 2 {
		log.Fatalf("Can't decode mapping for %q", n2k)
	} else {
		kIn, nOut = k2ns[0], k2ns[1]
	}
	log.Printf("Mapping Inbound NATS Subject %q to Kafka Topic: %q", nIn, kOut)
	log.Printf("Mapping Inbound Kafka Topic: %q to NATS Subject: %q", kIn, nOut)
	return
}
