/*
 * Copyright 2019-2022 The NATS Authors
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

package kafka

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/riferrei/srclient"
	"github.com/stretchr/testify/assert"
)

func TestRefineKeyWithSubjectHeader(t *testing.T) {
	withSubject := func(subject string) []sarama.RecordHeader {
		return []sarama.RecordHeader{
			{Key: []byte("num_pending"), Value: []byte("0")},
			{Key: []byte(subjectHeaderKey), Value: []byte(subject)},
		}
	}

	tests := []struct {
		name     string
		key      []byte
		headers  []sarama.RecordHeader
		expected []byte
	}{
		{
			name:     "inserts car_id from subject header",
			key:      []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
			headers:  withSubject("STATE.GLOBAL_V2C.19.CELL2.01-244530648.dyn.foo"),
			expected: []byte("STATE.GLOBAL_V2C.19.CELL2.01-244530648.>"),
		},
		{
			name:     "works when subject ends exactly at car_id",
			key:      []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
			headers:  withSubject("STATE.GLOBAL_V2C.19.CELL2.01-244530648"),
			expected: []byte("STATE.GLOBAL_V2C.19.CELL2.01-244530648.>"),
		},
		{
			name:     "deployment prefix BULK is preserved",
			key:      []byte("BULK.GLOBAL_V2C.19.CELL2.>"),
			headers:  withSubject("BULK.GLOBAL_V2C.19.CELL2.01-244530648.dyn"),
			expected: []byte("BULK.GLOBAL_V2C.19.CELL2.01-244530648.>"),
		},
		{
			name:     "no subject header keeps original key",
			key:      []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
			headers:  []sarama.RecordHeader{{Key: []byte("num_pending"), Value: []byte("0")}},
			expected: []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
		},
		{
			name:     "nil headers keeps original key",
			key:      []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
			headers:  nil,
			expected: []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
		},
		{
			name:     "key without trailing wildcard is unchanged",
			key:      []byte("STATE.GLOBAL_V2C.19.CELL2.01-244530648"),
			headers:  withSubject("STATE.GLOBAL_V2C.19.CELL2.01-244530648.dyn"),
			expected: []byte("STATE.GLOBAL_V2C.19.CELL2.01-244530648"),
		},
		{
			name:     "subject too short keeps original key",
			key:      []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
			headers:  withSubject("STATE.GLOBAL_V2C.19.CELL2"),
			expected: []byte("STATE.GLOBAL_V2C.19.CELL2.>"),
		},
		{
			name:     "empty key is unchanged",
			key:      []byte(""),
			headers:  withSubject("STATE.GLOBAL_V2C.19.CELL2.01-244530648"),
			expected: []byte(""),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, refineKeyWithSubjectHeader(tc.key, tc.headers))
		})
	}
}

func TestSerializePayloadAvro(t *testing.T) {
	server := newMockSchemaServer(t)
	defer server.close()

	producer := &saramaProducer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srclient.CreateSchemaRegistryClient(server.getServerURL()),
		subjectName:          avroSubjectName,
		schemaVersion:        avroSchemaVersion,
		schemaType:           srclient.Avro,
	}

	_, err := producer.serializePayload([]byte(avroMessage))
	assert.Nil(t, err)
}

func TestSerializePayloadJson(t *testing.T) {
	server := newMockSchemaServer(t)
	defer server.close()

	producer := &saramaProducer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srclient.CreateSchemaRegistryClient(server.getServerURL()),
		subjectName:          jsonSubjectName,
		schemaVersion:        jsonSchemaVersion,
		schemaType:           srclient.Json,
	}

	_, err := producer.serializePayload([]byte(jsonMessage))
	assert.Nil(t, err)
}

func TestSerializePayloadProtobuf(t *testing.T) {
	server := newMockSchemaServer(t)
	defer server.close()
	srClient := srclient.CreateSchemaRegistryClient(server.getServerURL())

	producer := &saramaProducer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srClient,
		subjectName:          protobufSubjectName,
		schemaVersion:        protobufSchemaVersion,
		schemaType:           srclient.Protobuf,
		pbSerializer:         newSerializer(),
	}
	schema, err := srClient.GetSchema(protobufSchemaID)
	assert.Nil(t, err)

	message, err := producer.serializePayload([]byte(protobufMessage))
	assert.Nil(t, err)

	_, err = newDeserializer().Deserialize(schema, message[5:])
	assert.Nil(t, err)
}
