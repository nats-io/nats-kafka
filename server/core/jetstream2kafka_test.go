/*
 * Copyright 2019-2021 The NATS Authors
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
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/require"
)

func TestSimpleSendOnJetStreamReceiveOnKafka(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	_, err = tbs.JS.Publish(subject, []byte(msg))
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
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestSimpleSASLSendOnJetStreamReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
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

	_, err = tbs.JS.Publish(subject, []byte(msg))
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
	require.Equal(t, int64(len([]byte(msg))), connStats.BytesOut)
	require.Equal(t, int64(1), connStats.Connects)
	require.Equal(t, int64(0), connStats.Disconnects)
	require.True(t, connStats.Connected)
}

func TestJetStreamQueueStartAtPosition(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"
	msg2 := "goodbye world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "JetStreamToKafka",
			Subject:         subject,
			Topic:           topic,
			StartAtSequence: 2,
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     nuid.Next(),
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	// Send 2 messages, should only get 2nd
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)
	_, err = tbs.JS.Publish(subject, []byte(msg2))
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

func TestSASLJetStreamQueueStartAtPosition(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"
	msg2 := "goodbye world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "JetStreamToKafka",
			Subject:         subject,
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

	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     nuid.Next(),
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	// Send 2 messages, should only get 2nd
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)
	_, err = tbs.JS.Publish(subject, []byte(msg2))
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

func TestJetStreamQueueDeliverLatest(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "JetStreamToKafka",
			Subject:         subject,
			Topic:           topic,
			StartAtSequence: -1,
		},
	}

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     nuid.Next(),
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	// Send 2 messages, should only get 2nd
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// Should get the last one
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)

	_, err = tbs.JS.Publish(subject, []byte(msg))
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

func TestJetStreamSASLQueueDeliverLatest(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:            "JetStreamToKafka",
			Subject:         subject,
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

	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     nuid.Next(),
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	// Send 2 messages, should only get 2nd
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	// Should get the last one
	_, _, _, err = tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)

	_, err = tbs.JS.Publish(subject, []byte(msg))
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

func TestJetStreamQueueStartAtTime(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     nuid.Next(),
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	// Send 2 messages, should only get 2nd
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	time.Sleep(2 * time.Second) // move the time along

	connect := []conf.ConnectorConfig{
		{
			Type:        "JetStreamToKafka",
			Subject:     subject,
			Topic:       topic,
			StartAtTime: time.Now().Unix(),
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // move the time along

	_, err = tbs.JS.Publish(subject, []byte(msg))
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

func TestJetStreamSASLQueueStartAtTime(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	tbs, err := StartTestEnvironmentInfrastructure(true, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     nuid.Next(),
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	// Send 2 messages, should only get 2nd
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)
	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	time.Sleep(2 * time.Second) // move the time along

	connect := []conf.ConnectorConfig{
		{
			Type:        "JetStreamToKafka",
			Subject:     subject,
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

	_, err = tbs.JS.Publish(subject, []byte(msg))
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

func TestJetStreamQueueDurableSubscriber(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	tbs, err := StartTestEnvironmentInfrastructure(false, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	stream := nuid.Next()
	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{subject},
	})
	require.NoError(t, err)
	durable := nuid.Next()
	_, err = tbs.JS.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:        durable,
		AckPolicy:      nats.AckExplicitPolicy,
		DeliverSubject: "foo",
	})
	require.NoError(t, err)

	connect := []conf.ConnectorConfig{
		{
			Type:            "JetStreamToKafka",
			Subject:         subject,
			Topic:           topic,
			DurableName:     durable,
			StartAtSequence: 2,
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	_, err = tbs.JS.Publish(subject, []byte("one"))
	require.NoError(t, err)

	tbs.WaitForRequests(1) // get that request through the system

	tbs.StopBridge()

	_, err = tbs.JS.Publish(subject, []byte("two"))
	require.NoError(t, err)

	_, err = tbs.JS.Publish(subject, []byte("three"))
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

func TestJetStreamSASLQueueDurableSubscriber(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()

	tbs, err := StartTestEnvironmentInfrastructure(true, false, []string{topic})
	require.NoError(t, err)
	defer tbs.Close()

	stream := nuid.Next()
	_, err = tbs.JS.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{subject},
	})
	require.NoError(t, err)
	durable := nuid.Next()
	_, err = tbs.JS.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:        durable,
		AckPolicy:      nats.AckExplicitPolicy,
		DeliverSubject: "foo",
	})
	require.NoError(t, err)

	connect := []conf.ConnectorConfig{
		{
			Type:            "JetStreamToKafka",
			Subject:         subject,
			Topic:           topic,
			DurableName:     durable,
			StartAtSequence: 2,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	err = tbs.StartBridge(connect)
	require.NoError(t, err)

	_, err = tbs.JS.Publish(subject, []byte("one"))
	require.NoError(t, err)

	tbs.WaitForRequests(1) // get that request through the system

	tbs.StopBridge()

	_, err = tbs.JS.Publish(subject, []byte("two"))
	require.NoError(t, err)

	_, err = tbs.JS.Publish(subject, []byte("three"))
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

func TestSimpleSendOnJetStreamReceiveOnKafkaWithTLS(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTLSTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
}

func TestFixedKeyFromJetStream(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "JetStreamToKafka",
			Subject:  subject,
			Topic:    topic,
			KeyType:  "fixed",
			KeyValue: "alpha",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSASLFixedKeyFromJetStream(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "JetStreamToKafka",
			Subject:  subject,
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

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSubjectKeyFromJetStream(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: subject,
			Topic:   topic,
			KeyType: "subject",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, subject, string(key))
}

func TestSASLSubjectKeyFromJetStream(t *testing.T) {
	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: subject,
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

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, subject, string(key))
}

func TestSubjectRegexKeyFromJetStream(t *testing.T) {
	prefix := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "JetStreamToKafka",
			Subject:  fmt.Sprintf("%s.alpha", prefix),
			Topic:    topic,
			KeyType:  "subjectre",
			KeyValue: fmt.Sprintf("%s\\.([^.]+)", prefix),
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(fmt.Sprintf("%s.alpha", prefix), []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSASLSubjectRegexKeyFromJetStream(t *testing.T) {
	prefix := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "JetStreamToKafka",
			Subject:  fmt.Sprintf("%s.alpha", prefix),
			Topic:    topic,
			KeyType:  "subjectre",
			KeyValue: fmt.Sprintf("%s\\.([^.]+)", prefix),
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(fmt.Sprintf("%s.alpha", prefix), []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestReplyKeyFromJetStream(t *testing.T) {
	subject := nuid.Next()
	durable := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:        "JetStreamToKafka",
			Subject:     subject,
			DurableName: durable,
			Topic:       topic,
			KeyType:     "reply",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, durable, string(key))
}

func TestSASLReplyKeyFromJetStream(t *testing.T) {
	subject := nuid.Next()
	durable := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:        "JetStreamToKafka",
			Subject:     subject,
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

	_, err = tbs.JS.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, _, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, durable, string(key))
}

func TestJetStreamAlreadyConnected(t *testing.T) {
	connect := []conf.ConnectorConfig{
		{
			Type:    "JetStreamToKafka",
			Subject: nuid.Next(),
			Topic:   nuid.Next(),
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	require.NoError(t, tbs.Bridge.connectToJetStream())
}
