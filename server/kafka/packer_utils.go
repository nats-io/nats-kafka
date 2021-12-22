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

import "unsafe"

// Utility functions to use with cmap.ConcurrentMap
func packInt32InString(inputNum int32) string {
	size := int(unsafe.Sizeof(inputNum))
	buffer := make([]byte, size)
	for i := 0; i < size; i++ {
		buffer[i] = *(*uint8)(unsafe.Pointer(uintptr(unsafe.Pointer(&inputNum)) + uintptr(i)))
	}

	return string(buffer)
}

func packIntInString(inputNum int) string {
	size := int(unsafe.Sizeof(inputNum))
	buffer := make([]byte, size)
	for i := 0; i < size; i++ {
		buffer[i] = *(*uint8)(unsafe.Pointer(uintptr(unsafe.Pointer(&inputNum)) + uintptr(i)))
	}

	return string(buffer)
}

func unpackInt32FromString(inputString string) int32 {
	outputValue := int32(0)
	inputBytes := []byte(inputString)
	size := len(inputBytes)
	for i := 0; i < size; i++ {
		*(*uint8)(unsafe.Pointer(uintptr(unsafe.Pointer(&outputValue)) + uintptr(i))) = inputBytes[i]
	}

	return outputValue
}

func unpackIntFromString(inputString string) int {
	outputValue := 0
	inputBytes := []byte(inputString)
	size := len(inputBytes)
	for i := 0; i < size; i++ {
		*(*uint8)(unsafe.Pointer(uintptr(unsafe.Pointer(&outputValue)) + uintptr(i))) = inputBytes[i]
	}

	return outputValue
}
