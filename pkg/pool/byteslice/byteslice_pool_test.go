// Copyright (c) 2023 Paweł Gaczyński
// Copyright (c) 2021 Andy Pan
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

package byteslice

import (
	"runtime/debug"
	"testing"

	. "github.com/stretchr/testify/require"
)

func TestByteSlice(t *testing.T) {
	buf := Get(8)
	copy(buf, "ff")

	if string(buf[:2]) != "ff" {
		t.Fatal("expect copy result is ff, but not")
	}

	// Disable GC to test re-acquire the same data
	gc := debug.SetGCPercent(-1)

	Put(buf)

	newBuf := Get(7)
	if &newBuf[0] != &buf[0] {
		t.Fatal("expect newBuf and buf to be the same array")
	}

	if string(newBuf[:2]) != "ff" {
		t.Fatal("expect the newBuf is the buf, but not")
	}

	// Re-enable GC
	debug.SetGCPercent(gc)
}

func BenchmarkByteSlice(b *testing.B) {
	b.Run("Run.N", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bs := Get(1024)
			Put(bs)
		}
	})
	b.Run("Run.Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bs := Get(1024)
				Put(bs)
			}
		})
	})
}

func TestByteSlicePool(t *testing.T) {
	pool := NewByteSlicePool()

	pool.Put(make([]byte, 0))

	Nil(t, pool.Get(-1))
	Nil(t, pool.Get(0))
}
