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

	"github.com/stretchr/testify/assert"
)

func TestPackInt32InString(t *testing.T) {
	op, err := packInt32InString(2)
	assert.Nil(t, err)
	assert.Equal(t, "\x00\x00\x00\x02", op)
}

func TestUnpackInt32FromString(t *testing.T) {
	op, err := unpackInt32FromString("\x00\x00\x00\x02")
	assert.Nil(t, err)
	assert.Equal(t, int32(2), op)
}
