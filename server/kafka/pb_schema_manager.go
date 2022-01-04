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
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/riferrei/srclient"
)

var once sync.Once

var (
	schemaManager protobufSchemaManager
)

type protobufSchemaManager struct {
	protobufSchemaIDtoFDMappings cmap.ConcurrentMap // schema id to desc.FileDescriptor map
}

func newProtobufSchemaManager() protobufSchemaManager {
	once.Do(func() {
		schemaManager = protobufSchemaManager{
			protobufSchemaIDtoFDMappings: cmap.New(),
		}
	})

	return schemaManager
}

func (protobufSchemaManager) getFileDescriptor(schema *srclient.Schema) (*desc.FileDescriptor, error) {
	packedSchemaID := strconv.Itoa(schema.ID())
	if !schemaManager.protobufSchemaIDtoFDMappings.Has(packedSchemaID) {
		errorReporter := func(err protoparse.ErrorWithPos) error {
			position := err.GetPosition()
			return fmt.Errorf("unable to parse file descriptor %s %d: %w", position.Filename, position.Line, err.Unwrap())
		}

		nanoTs := strconv.FormatInt(time.Now().UnixNano(), 10)
		schemaFileName := packedSchemaID + "-" + nanoTs + ".proto"
		schemaFile, err := os.CreateTemp("", schemaFileName)
		if err != nil {
			return nil, err
		}
		_, err = schemaFile.WriteString(schema.Schema())
		if err != nil {
			return nil, err
		}
		err = schemaFile.Close()
		if err != nil {
			return nil, err
		}
		defer os.Remove(schemaFile.Name())

		schemaMap := make(map[string]string, 1)
		schemaMap[schemaFileName] = schema.Schema()
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
		schemaManager.protobufSchemaIDtoFDMappings.Set(packedSchemaID, fds[0])
	}

	fd, _ := schemaManager.protobufSchemaIDtoFDMappings.Get(packedSchemaID)
	return fd.(*desc.FileDescriptor), nil
}

func (protobufSchemaManager) getMessageDescriptor(schema *srclient.Schema) (*desc.MessageDescriptor, error) {
	fd, err := schemaManager.getFileDescriptor(schema)
	if err != nil {
		return nil, err
	}

	return fd.GetMessageTypes()[0], nil
}
