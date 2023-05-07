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

package virtualmem

import (
	"math/bits"

	"github.com/pawelgaczynski/gain/pkg/pool/sync"
)

const (
	maxVMSize = 64 * 1024 * 1024
)

var builtinPool = NewPool()

func Get(size int) *VirtualMem {
	return builtinPool.Get(size)
}

// Put returns the virtual memory to the built-in pool.
func Put(mem *VirtualMem) {
	mem.Zeroes()
	builtinPool.Put(mem)
}

// Get retrieves a byte slice of the length requested by the caller from pool or allocates a new one.
func (p *Pool) Get(size int) *VirtualMem {
	if size <= 0 {
		return nil
	}
	size = AdjustBufferSize(size)

	if size > maxVMSize {
		return NewVirtualMem(size)
	}
	idx := index(uint32(size))

	ptr := p.pools[idx].Get()
	if ptr == nil {
		return NewVirtualMem(size)
	}

	return ptr
}

// Put returns the virtual memory to the pool.
func (p *Pool) Put(mem *VirtualMem) {
	if mem.Size < pageSize || mem.Size > maxVMSize {
		return
	}

	idx := index(uint32(mem.Size))
	if mem.Size != 1<<idx { // this byte slice is not from Pool.Get(), put it into the previous interval of idx
		idx--
	}

	p.pools[idx].Put(mem)
}

func index(n uint32) uint32 {
	return uint32(bits.Len32(n - 1))
}

type Pool struct {
	pools [32]sync.Pool[*VirtualMem]
}

func NewPool() Pool {
	var pool Pool
	for i := range pool.pools {
		pool.pools[i] = sync.NewPool[*VirtualMem]()
	}

	return pool
}
