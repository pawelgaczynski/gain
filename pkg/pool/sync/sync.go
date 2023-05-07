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

package sync

import (
	"sync"
	"sync/atomic"
)

type Pool[T any] interface {
	Get() T
	Put(T)
}

type pool[T any] struct {
	internalPool sync.Pool
	count        int64
}

func getZero[T any]() T {
	var result T

	return result
}

func (p *pool[T]) Get() T {
	val, ok := p.internalPool.Get().(T)
	if !ok {
		return getZero[T]()
	}

	atomic.AddInt64(&p.count, -1)

	return val
}

func (p *pool[T]) Put(value T) {
	p.internalPool.Put(value)
	atomic.AddInt64(&p.count, 1)
}

func NewPool[T any]() Pool[T] {
	return &pool[T]{
		internalPool: sync.Pool{},
	}
}
