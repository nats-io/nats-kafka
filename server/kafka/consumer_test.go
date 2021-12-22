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
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"

	"github.com/linkedin/goavro/v2"
	"github.com/riferrei/srclient"
	"github.com/stretchr/testify/assert"
)

func TestDeserializePayloadAvro(t *testing.T) {
	server := newMockSchemaServer(t)
	defer server.close()

	consumer := saramaConsumer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srclient.CreateSchemaRegistryClient(server.getServerURL()),
		schemaType:           srclient.Avro,
	}

	avroCodec, err := goavro.NewCodec(avroSchema)
	assert.Nil(t, err)
	native, _, err := avroCodec.NativeFromTextual([]byte(avroMessage))
	assert.Nil(t, err)
	avroBytes, err := avroCodec.BinaryFromNative(nil, native)
	assert.Nil(t, err)

	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(avroSchemaID))

	var payload []byte
	payload = append(payload, byte(0))
	payload = append(payload, schemaIDBytes...)
	payload = append(payload, avroBytes...)

	_, err = consumer.deserializePayload(payload)
	assert.Nil(t, err)
}

func TestDeserializePayloadJson(t *testing.T) {
	server := newMockSchemaServer(t)
	defer server.close()

	consumer := &saramaConsumer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srclient.CreateSchemaRegistryClient(server.getServerURL()),
		schemaType:           srclient.Json,
	}

	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(jsonSchemaID))

	var payload []byte
	payload = append(payload, byte(0))
	payload = append(payload, schemaIDBytes...)
	payload = append(payload, []byte(jsonMessage)...)

	_, err := consumer.deserializePayload(payload)
	assert.Nil(t, err)
}

func TestDeserializePayloadProtobuf(t *testing.T) {
	server := newMockSchemaServer(t)
	defer server.close()

	consumer := &saramaConsumer{
		schemaRegistryOn:     true,
		schemaRegistryClient: srclient.CreateSchemaRegistryClient(server.getServerURL()),
		schemaType:           srclient.Protobuf,
		pbDeserializer:       newDeserializer(),
	}

	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(protobufSchemaID))

	msgIndexBytes := make([]byte, 16)
	length := binary.PutVarint(msgIndexBytes, 1)
	binary.PutVarint(msgIndexBytes[length:], 0)

	protoBytes, err := createProtobufMessage()
	assert.Nil(t, err)

	var payload []byte
	payload = append(payload, byte(0))
	payload = append(payload, schemaIDBytes...)
	payload = append(payload, msgIndexBytes...)
	payload = append(payload, protoBytes...)

	_, err = consumer.deserializePayload(payload)
	assert.Nil(t, err)
}

func createProtobufMessage() ([]byte, error) {
	errorReporter := func(err protoparse.ErrorWithPos) error {
		position := err.GetPosition()
		return fmt.Errorf("unable to parse file descriptor %s %d: %w", position.Filename, position.Line, err.Unwrap())
	}

	nanoTs := strconv.FormatInt(time.Now().UnixNano(), 10)
	schemaFileName := "test-" + nanoTs + ".proto"
	file, err := os.CreateTemp("", schemaFileName)
	if err != nil {
		return nil, err
	}

	_, err = file.WriteString(protobufSchema)
	if err != nil {
		return nil, err
	}

	err = file.Close()
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())

	schemaMap := make(map[string]string, 1)
	schemaMap[schemaFileName] = protobufSchema
	var schemaFilePaths []string
	schemaFilePaths = append(schemaFilePaths, schemaFileName)
	protobufParser := &protoparse.Parser{
		Accessor:              protoparse.FileContentsFromMap(schemaMap),
		ImportPaths:           []string{"."},
		InferImportPaths:      true,
		ValidateUnlinkedFiles: true,
		ErrorReporter:         errorReporter,
	}
	fds, err := protobufParser.ParseFiles(schemaFilePaths...)
	if err != nil {
		return nil, err
	}

	dynamicMessage := dynamic.NewMessage(fds[0].GetMessageTypes()[0])
	err = dynamicMessage.UnmarshalJSON([]byte(protobufMessage))
	if err != nil {
		return nil, err
	}

	bytes, err := dynamicMessage.Marshal()
	if err != nil {
		return nil, err
	}

	return bytes, nil
}
