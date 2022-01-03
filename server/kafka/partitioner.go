/*
 * Copyright 2020 The NATS Authors
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
	"github.com/Shopify/sarama"
	cmap "github.com/orcaman/concurrent-map"
)

// A balancer (or partitioner in Sarama terms) is in charge of spreading messages across available partitions of a topic.
// Sarama provides a hash based partitioner which is default in the producer. This implementation distributes messages
// based on number of bytes each partition has received.
type leastBytesPartitioner struct {
	byteCounters cmap.ConcurrentMap
}

// NewLeastBytesPartitioner function takes topic as an argument, but it is not used. This has been done as it
// implements the sarama.PartitionerConstructor interface which requires it.
func NewLeastBytesPartitioner(topic string) sarama.Partitioner {
	lbp := new(leastBytesPartitioner)
	lbp.byteCounters = cmap.New()
	return lbp
}

func (lbp *leastBytesPartitioner) RequiresConsistency() bool {
	return false
}

func (lbp *leastBytesPartitioner) Partition(message *sarama.ProducerMessage, numPartitions int32) (int32, error) {
	// if partition count has reduced, remove the old entries
	for i := int32(lbp.byteCounters.Count() - 1); i >= numPartitions; i-- {
		lbp.byteCounters.Remove(packInt32InString(i))
	}

	// if the size has increased, add counters for new partitions
	for i := int32(lbp.byteCounters.Count()); i < numPartitions; i++ {
		lbp.byteCounters.Set(packInt32InString(i), uint64(0))
	}

	// find the entry in the byteCounters with min bytes
	minIndex := lbp.findPartitionWithMinBytes(lbp.byteCounters)
	minBytes, _ := lbp.byteCounters.Get(minIndex)
	lbp.byteCounters.Set(minIndex, minBytes.(uint64)+uint64(message.Key.Length())+uint64(message.Value.Length()))

	return unpackInt32FromString(minIndex), nil
}

func (lbp *leastBytesPartitioner) findPartitionWithMinBytes(counters cmap.ConcurrentMap) string {
	var minPartition string
	var minBytes uint64

	for entry := range counters.IterBuffered() {
		curBytes, _ := entry.Val.(uint64)
		if minBytes == 0 || curBytes < minBytes {
			minBytes = curBytes
			minPartition = entry.Key
		}
	}

	return minPartition
}
