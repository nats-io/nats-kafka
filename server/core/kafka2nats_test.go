/*
 * Copyright 2019-2020 The NATS Authors
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
 */

package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/Shopify/sarama"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/require"
)

func TestSimpleSendOnKafkaReceiveOnNATS(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	done := make(chan string)

	sub, err := tbs.NC.Subscribe(subject, func(msg *nats.Msg) {
		done <- string(msg.Data)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = tbs.SendMessageToKafka(topic, []byte(msg), 5000)
	require.NoError(t, err)

	received := tbs.WaitForIt(1, done)
	require.Equal(t, msg, received)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestSimpleSendOnKafkaReceiveOnNATSWithHeader(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
		},
	}

	var kHdrs = make([]sarama.RecordHeader, 3)
	i := 0
	for i < len(kHdrs) {
		keyString := fmt.Sprintf("key-%d", i)
		valueString := fmt.Sprintf("value-%d", i)
		kHdrs[i].Key = []byte(keyString)
		kHdrs[i].Value = []byte(valueString)
		i++
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	done := make(chan *nats.Msg)

	sub, err := tbs.NC.Subscribe(subject, func(msg *nats.Msg) {
		done <- msg
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = tbs.SendMessageWithHeadersToKafka(topic, []byte(msg), kHdrs, 5000)
	require.NoError(t, err)

	received := tbs.WaitForNatsMsg(1, done)
	require.Equal(t, msg, string(received.Data))

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)

	i = 0
	require.Equal(t, 3, len(received.Header))
	for nKey, nValue := range received.Header {
		//fmt.Printf("%s->%s", nKey, nValue)
		require.Equal(t, string(kHdrs[i].Key), nKey)
		require.Equal(t, string(kHdrs[i].Value), nValue[0])
		i++
	}
}

func TestSimpleSASLSendOnKafkaReceiveOnNATS(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	done := make(chan string)

	sub, err := tbs.NC.Subscribe(subject, func(msg *nats.Msg) {
		done <- string(msg.Data)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = tbs.SendMessageToKafka(topic, []byte(msg), 5000)
	require.NoError(t, err)

	received := tbs.WaitForIt(1, done)
	require.Equal(t, msg, received)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestSimpleSendOnKafkaReceiveOnNATSWithGroup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	group := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
			GroupID: group,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	// This test almost always fails if we don't wait a bit here.
	// It has probably something to do with some initialization in Kafka,
	// but I am not sure what. This delay is not bullet proof but at least helps.
	time.Sleep(time.Second)

	done := make(chan string)

	sub, err := tbs.NC.Subscribe(subject, func(msg *nats.Msg) {
		done <- string(msg.Data)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = tbs.SendMessageToKafka(topic, []byte(msg), 5000)
	require.NoError(t, err)

	received := tbs.WaitForIt(1, done)
	require.Equal(t, msg, received)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestSimpleSASLSendOnKafkaReceiveOnNATSWithGroup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	group := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
			GroupID: group,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	done := make(chan string)

	sub, err := tbs.NC.Subscribe(subject, func(msg *nats.Msg) {
		done <- string(msg.Data)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = tbs.SendMessageToKafka(topic, []byte(msg), 5000)
	require.NoError(t, err)

	received := tbs.WaitForIt(1, done)
	require.Equal(t, msg, received)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestSimpleSendOnQueueReceiveOnNatsWithTLS(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "KafkaToNATS",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTLSTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	done := make(chan string)

	sub, err := tbs.NC.Subscribe(subject, func(msg *nats.Msg) {
		done <- string(msg.Data)
	})
	require.NoError(t, err)
	tbs.NC.Flush()
	defer sub.Unsubscribe()

	err = tbs.SendMessageToKafka(topic, []byte(msg), 5000)
	require.NoError(t, err)

	received := tbs.WaitForIt(1, done)
	require.Equal(t, msg, received)
}
