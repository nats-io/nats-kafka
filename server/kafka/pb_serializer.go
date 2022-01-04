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
	"bytes"
	"encoding/binary"
	"strings"
	"unsafe"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/riferrei/srclient"
)

type pbSerializer interface {
	Serialize(*srclient.Schema, []byte) ([]byte, error)
}

type protobufSerializer struct {
	schemaManager protobufSchemaManager
}

func newSerializer() pbSerializer {
	return &protobufSerializer{
		schemaManager: newProtobufSchemaManager(),
	}
}

func (ps *protobufSerializer) Serialize(schema *srclient.Schema, payload []byte) ([]byte, error) {
	// Get the message descriptor from cache or build it
	messageDescriptor, err := schemaManager.getMessageDescriptor(schema)
	if err != nil {
		return nil, err
	}

	// Parse the protobuf json sent as payload and convert it into wire format
	message := dynamic.NewMessage(messageDescriptor)
	err = message.UnmarshalJSON(payload)
	if err != nil {
		return nil, err
	}

	indexBytes, err := ps.buildMessageIndexes(schema, messageDescriptor.GetFullyQualifiedName())
	if err != nil {
		return nil, err
	}

	protoBytes, err := message.Marshal()
	if err != nil {
		return nil, err
	}

	serializedPayload := make([]byte, len(indexBytes)+len(protoBytes)+16) // 16 extra bytes for the array length
	binary.PutVarint(serializedPayload, int64(len(indexBytes)/int(unsafe.Sizeof(int32(0)))))
	if len(indexBytes) > 0 {
		serializedPayload = append(serializedPayload, indexBytes...)
	}
	serializedPayload = append(serializedPayload, protoBytes...)
	return serializedPayload, nil
}

func (ps *protobufSerializer) buildMessageIndexes(schema *srclient.Schema, name string) ([]byte, error) {
	fileDescriptor, err := schemaManager.getFileDescriptor(schema)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(name, ".")
	messageTypes := fileDescriptor.GetMessageTypes()

	var indexes []byte
	for _, part := range parts {
		i := int32(0)
		for _, mType := range messageTypes {
			if mType.GetName() == part {
				indexBuf := new(bytes.Buffer)
				err = binary.Write(indexBuf, binary.BigEndian, i)
				if err != nil {
					return nil, err
				}

				indexes = append(indexes, indexBuf.Bytes()...)
				break
			}
			i++
		}
	}

	return indexes, nil
}
