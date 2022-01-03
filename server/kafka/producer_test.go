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

	"github.com/riferrei/srclient"
	"github.com/stretchr/testify/assert"
)

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

	producer := &saramaProducer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srclient.CreateSchemaRegistryClient(server.getServerURL()),
		subjectName:          protobufSubjectName,
		schemaVersion:        protobufSchemaVersion,
		schemaType:           srclient.Protobuf,
		pbSerializer:         newSerializer(),
	}

	_, err := producer.serializePayload([]byte(protobufMessage))
	assert.Nil(t, err)
}
