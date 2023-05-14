// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ringbuffer

import (
	"testing"

	. "github.com/stretchr/testify/require"
)

func TestIndexRingBufferPool(t *testing.T) {
	idx := indexRingBufferPool(64)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(128)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(256)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(512)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(1024)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(2048)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(4096)
	Equal(t, 0, idx)
	idx = indexRingBufferPool(8192)
	Equal(t, 1, idx)
	idx = indexRingBufferPool(16384)
	Equal(t, 2, idx)
	idx = indexRingBufferPool(32768)
	Equal(t, 3, idx)
	idx = indexRingBufferPool(65536)
	Equal(t, 4, idx)
	idx = indexRingBufferPool(131072)
	Equal(t, 5, idx)
	idx = indexRingBufferPool(262144)
	Equal(t, 6, idx)
	idx = indexRingBufferPool(524288)
	Equal(t, 7, idx)
	idx = indexRingBufferPool(1048576)
	Equal(t, 8, idx)
	idx = indexRingBufferPool(2097152)
	Equal(t, 9, idx)
	idx = indexRingBufferPool(4194304)
	Equal(t, 10, idx)
	idx = indexRingBufferPool(8388608)
	Equal(t, 11, idx)
	idx = indexRingBufferPool(16777216)
	Equal(t, 12, idx)
	idx = indexRingBufferPool(33554432)
	Equal(t, 13, idx)
	idx = indexRingBufferPool(67108864)
	Equal(t, 14, idx)
	idx = indexRingBufferPool(134217728)
	Equal(t, 14, idx)
}
