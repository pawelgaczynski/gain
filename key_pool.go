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

package gain

import (
	"sync/atomic"

	"github.com/pawelgaczynski/gain/pkg/stack"
)

const firstFreeKey = 2

type keyPool struct {
	stack   stack.LockFreeStack[uint64]
	nextKey uint64
}

func (p *keyPool) get() uint64 {
	key := p.stack.Pop()
	if key == 0 {
		value := atomic.AddUint64(&p.nextKey, 1) - 1

		return value
	}

	return key
}

func (p *keyPool) put(key uint64) {
	p.stack.Push(key)
}

func newKeyPool() *keyPool {
	return &keyPool{
		stack: stack.NewLockFreeStack[uint64](),
		// 0 is invalid key, 1 is reserved for main socket
		nextKey: firstFreeKey,
	}
}
