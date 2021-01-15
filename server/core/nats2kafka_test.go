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
 */

package core

import (
	"testing"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/require"
)

func TestSimpleSendOnNatsReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := "test"
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	tbs.Bridge.checkConnections()

	err = tbs.NC.Publish("test", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
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

func TestSimpleSASLSendOnNatsReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := "test"
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
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

	err = tbs.NC.Publish("test", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
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

func TestWildcardSendRecieveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Topic:   topic,
			Subject: "test.*",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish("test.a", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
}

func TestWildcardSASLSendRecieveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Topic:   topic,
			Subject: "test.*",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish("test.a", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
}

func TestSendOnNatsQueueReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := "test"
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:      "NATSToKafka",
			Subject:   subject,
			QueueName: "workers",
			Topic:     topic,
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish("test", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
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

func TestSASLSendOnNatsQueueReceiveOnKafka(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := "test"
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:      "NATSToKafka",
			Subject:   subject,
			QueueName: "workers",
			Topic:     topic,
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish("test", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
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

func TestSimpleSendOnNatsReceiveOnKafkaWithTLS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := "test"
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
		},
	}

	tbs, err := StartTLSTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish("test", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	_, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
}

func TestFixedKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "NATSToKafka",
			Subject:  subject,
			Topic:    topic,
			KeyType:  "fixed",
			KeyValue: "alpha",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSASLFixedKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "NATSToKafka",
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

	err = tbs.NC.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSubjectKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
			KeyType: "subject",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, subject, string(key))
}

func TestSASLSubjectKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
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

	err = tbs.NC.Publish(subject, []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, subject, string(key))
}

func TestReplyKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
			KeyType: "reply",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.PublishRequest(subject, "beta", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "beta", string(key))
}

func TestSASLReplyKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:    "NATSToKafka",
			Subject: subject,
			Topic:   topic,
			KeyType: "reply",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.PublishRequest(subject, "beta", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "beta", string(key))
}

func TestSubjectRegexKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "NATSToKafka",
			Subject:  subject + ".*", // need wildcard
			Topic:    topic,
			KeyType:  "subjectre",
			KeyValue: subject + "\\.([^.]+)",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish(subject+".alpha", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestSASLSubjectRegexKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "NATSToKafka",
			Subject:  subject + ".*", // need wildcard
			Topic:    topic,
			KeyType:  "subjectre",
			KeyValue: subject + "\\.([^.]+)",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.Publish(subject+".alpha", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "alpha", string(key))
}

func TestReplyRegexKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "NATSToKafka",
			Subject:  subject,
			Topic:    topic,
			KeyType:  "replyre",
			KeyValue: "beta\\.([^.]+)",
		},
	}

	tbs, err := StartTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.PublishRequest(subject, "beta.gamma", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "gamma", string(key))
}

func TestSASLReplyRegexKeyFromNATS(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	subject := nuid.Next()
	topic := nuid.Next()
	msg := "hello world"

	connect := []conf.ConnectorConfig{
		{
			Type:     "NATSToKafka",
			Subject:  subject,
			Topic:    topic,
			KeyType:  "replyre",
			KeyValue: "beta\\.([^.]+)",
			SASL: conf.SASL{
				User:     saslUser,
				Password: saslPassword,
			},
		},
	}

	tbs, err := StartSASLTestEnvironment(connect)
	require.NoError(t, err)
	defer tbs.Close()

	err = tbs.NC.PublishRequest(subject, "beta.gamma", []byte(msg))
	require.NoError(t, err)

	reader := tbs.CreateReader(topic, 5000)
	defer reader.Close()

	key, data, err := tbs.GetMessageFromKafka(reader, 5000)
	require.NoError(t, err)
	require.Equal(t, msg, string(data))
	require.Equal(t, "gamma", string(key))
}
