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

package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/require"
)

func TestSimpleSendOnStanReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: channel,
			Topic:   topic,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len(data)), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestSASLSimpleSendOnStanReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: channel,
			Topic:   topic,
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesIn)
	require.Equal(t, int64(len(data)), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestQueueStartAtPosition(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"
	msg2 := "goodbye world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "STANToKafka",
			Channel:         channel,
			Topic:           topic,
			StartAtSequence: 2,
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	// Send 2 messages, should only get 2nd
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)
	err = tbs.SC.Publish(channel, []byte(msg2))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg2, string(data))
	_, _, _, err = tbs.GetMessageFromKafka(reader, 2000)
	require.Error(t, err)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
}

func TestSASLQueueStartAtPosition(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"
	msg2 := "goodbye world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "STANToKafka",
			Channel:         channel,
			Topic:           topic,
			StartAtSequence: 2,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(true, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	// Send 2 messages, should only get 2nd
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)
	err = tbs.SC.Publish(channel, []byte(msg2))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg2, string(data))
	_, _, _, err = tbs.GetMessageFromKafka(reader, 2000)
	require.Error(t, err)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
}

func TestQueueDeliverLatest(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "STANToKafka",
			Channel:         channel,
			Topic:           topic,
			StartAtSequence: -1,
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	// Send 2 messages, should only get 2nd
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// Should get the last one
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	// Should receive 1 message we just sent
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 3000)
	require.Error(t, err)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(2), connStats.MessagesIn)
	require.Equal(t, int64(2), connStats.MessagesOut)
}

func TestSASLQueueDeliverLatest(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "STANToKafka",
			Channel:         channel,
			Topic:           topic,
			StartAtSequence: -1,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(true, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	// Send 2 messages, should only get 2nd
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// Should get the last one
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	// Should receive 1 message we just sent
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 3000)
	require.Error(t, err)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(2), connStats.MessagesIn)
	require.Equal(t, int64(2), connStats.MessagesOut)
}

func TestQueueStartAtTime(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	// Send 2 messages, should only get 2nd
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	time.Sleep(2 * time.Second) // move the time along

	connect := []conf.ConnectorConfig{
		{
			Type:        "STANToKafka",
			Channel:     channel,
			Topic:       topic,
			StartAtTime: time.Now().Unix(),
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // move the time along

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// Should only get the one we just sent
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 3000)
	require.Error(t, err)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
}

func TestSASLQueueStartAtTime(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	tbs, err := StartTestEnvironmentInfrastructure(true, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	// Send 2 messages, should only get 2nd
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)
	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	time.Sleep(2 * time.Second) // move the time along

	connect := []conf.ConnectorConfig{
		{
			Type:        "STANToKafka",
			Channel:     channel,
			Topic:       topic,
			StartAtTime: time.Now().Unix(),
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // move the time along

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// Should only get the one we just sent
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 3000)
	require.Error(t, err)

	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(1), connStats.MessagesIn)
	require.Equal(t, int64(1), connStats.MessagesOut)
}

func TestQueueDurableSubscriber(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	connect := []conf.ConnectorConfig{
		{
			Type:            "STANToKafka",
			Channel:         channel,
			Topic:           topic,
			DurableName:     nuid.Next(),
			StartAtSequence: 1,
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	err = tbs.SC.Publish(channel, []byte("one"))
	require.NoError(t, err)

	tbs.WaitForRequests(1) // get that request through the system

	tbs.StopBridge()

	err = tbs.SC.Publish(channel, []byte("two"))
	require.NoError(t, err)

	err = tbs.SC.Publish(channel, []byte("three"))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// should get three, even though we stopped the bridge
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 2000)
	require.Error(t, err)

	tbs.WaitForRequests(2) // get the later requests through the system

	// Should have 2 messages since the relaunch
	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(2), connStats.MessagesIn)
	require.Equal(t, int64(2), connStats.MessagesOut)
}

func TestSASLQueueDurableSubscriber(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()

	tbs, err := StartTestEnvironmentInfrastructure(true, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	connect := []conf.ConnectorConfig{
		{
			Type:            "STANToKafka",
			Channel:         channel,
			Topic:           topic,
			DurableName:     nuid.Next(),
			StartAtSequence: 1,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	err = tbs.SC.Publish(channel, []byte("one"))
	require.NoError(t, err)

	tbs.WaitForRequests(1) // get that request through the system

	tbs.StopBridge()

	err = tbs.SC.Publish(channel, []byte("two"))
	require.NoError(t, err)

	err = tbs.SC.Publish(channel, []byte("three"))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// should get three, even though we stopped the bridge
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	_, _, _, err = tbs.GetMessageFromKafka(reader, 2000)
	require.Error(t, err)

	tbs.WaitForRequests(2) // get the later requests through the system

	// Should have 2 messages since the relaunch
	stats := tbs.Bridge.SafeStats()
	connStats := stats.Connections[0]
	require.Equal(t, int64(2), connStats.MessagesIn)
	require.Equal(t, int64(2), connStats.MessagesOut)
}

func TestSimpleSendOnStanReceiveOnKafkaWithTLS(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: channel,
			Topic:   topic,
		},
	}

	tbs, err := StartTLSTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
}

func TestFixedKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "STANToKafka",
			Channel:  channel,
			Topic:    topic,
			KeyType:  "fixed",
			KeyValue: "alpha",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSASLFixedKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "STANToKafka",
			Channel:  channel,
			Topic:    topic,
			KeyType:  "fixed",
			KeyValue: "alpha",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSubjectKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: channel,
			Topic:   topic,
			KeyType: "subject",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, channel, string(key))
}

func TestSASLSubjectKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: channel,
			Topic:   topic,
			KeyType: "subject",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, channel, string(key))
}

func TestSubjectRegexKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "STANToKafka",
			Channel:  channel + ".alpha",
			Topic:    topic,
			KeyType:  "subjectre",
			KeyValue: channel + "\\.([^.]+)",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel+".alpha", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSASLSubjectRegexKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "STANToKafka",
			Channel:  channel + ".alpha",
			Topic:    topic,
			KeyType:  "subjectre",
			KeyValue: channel + "\\.([^.]+)",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel+".alpha", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestReplyKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	durable := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:        "STANToKafka",
			Channel:     channel,
			DurableName: durable,
			Topic:       topic,
			KeyType:     "reply",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, durable, string(key))
}

func TestSASLReplyKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	durable := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:        "STANToKafka",
			Channel:     channel,
			DurableName: durable,
			Topic:       topic,
			KeyType:     "reply",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, durable, string(key))
}

func TestReplyRegexKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	durable := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:        "STANToKafka",
			Channel:     channel,
			DurableName: durable + ".beta",
			Topic:       topic,
			KeyType:     "replyre",
			KeyValue:    durable + "\\.([^.]+)",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "beta", string(key))
}

func TestSASLReplyRegexKeyFromStan(t *testing.T) {
	channel := nuid.Next()
	durable := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:        "STANToKafka",
			Channel:     channel,
			DurableName: durable + ".beta",
			Topic:       topic,
			KeyType:     "replyre",
			KeyValue:    durable + "\\.([^.]+)",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.SC.Publish(channel, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "beta", string(key))
}

func TestSTANReconnectTimer(t *testing.T) {
	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: nuid.Next(),
			Topic:   nuid.Next(),
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.reconnectLock.Lock()
	tbs.Bridge.config.ReconnectInterval = 125
	sc1 := tbs.Bridge.stan
	tbs.Bridge.stan = nil
	tbs.Bridge.reconnectLock.Unlock()

	tbs.Bridge.checkConnections()

	time.Sleep(250 * time.Millisecond)

	tbs.Bridge.reconnectLock.Lock()
	sc2 := tbs.Bridge.stan
	tbs.Bridge.reconnectLock.Unlock()

	require.NotEqual(t, sc1, sc2)
}

func TestSTANConnectionLostClientIDRegistered(t *testing.T) {
	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: nuid.Next(),
			Topic:   nuid.Next(),
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.reconnectLock.Lock()
	tbs.Bridge.config.ReconnectInterval = 125
	sc1 := tbs.Bridge.stan
	tbs.Bridge.reconnectLock.Unlock()
	tbs.Bridge.stanConnectionLost(sc1, fmt.Errorf("lost connection"))

	time.Sleep(250 * time.Millisecond)

	tbs.Bridge.reconnectLock.Lock()
	sc2 := tbs.Bridge.stan
	tbs.Bridge.reconnectLock.Unlock()

	require.NotEqual(t, sc1, sc2)
}

func TestSTANConnectionLost(t *testing.T) {
	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: nuid.Next(),
			Topic:   nuid.Next(),
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.reconnectLock.Lock()
	tbs.Bridge.config.ReconnectInterval = 125
	sc1 := tbs.Bridge.stan
	tbs.Bridge.config.STAN.ClientID += "new"
	tbs.Bridge.reconnectLock.Unlock()
	tbs.Bridge.stanConnectionLost(sc1, fmt.Errorf("lost connection"))

	time.Sleep(250 * time.Millisecond)

	tbs.Bridge.reconnectLock.Lock()
	sc2 := tbs.Bridge.stan
	tbs.Bridge.reconnectLock.Unlock()

	require.NotEqual(t, sc1, sc2)
}

func TestSTANAlreadyConnected(t *testing.T) {
	connect := []conf.ConnectorConfig{
		{
			Type:    "STANToKafka",
			Channel: nuid.Next(),
			Topic:   nuid.Next(),
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	require.NoError(t, tbs.Bridge.connectToSTAN())
}
