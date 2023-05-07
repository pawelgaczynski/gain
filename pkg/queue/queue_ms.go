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

package queue

import (
	"sync/atomic"
	"unsafe"
)

type msQueue[T any] struct {
	head      unsafe.Pointer
	tail      unsafe.Pointer
	queueSize int32
}

type node[T any] struct {
	value *T
	next  unsafe.Pointer
}

func NewQueue[T any]() LockFreeQueue[T] {
	n := unsafe.Pointer(&node[T]{})

	return &msQueue[T]{head: n, tail: n}
}

func NewIntQueue() LockFreeQueue[int] {
	n := unsafe.Pointer(&node[int]{})

	return &msQueue[int]{head: n, tail: n}
}

func (q *msQueue[T]) Enqueue(value T) {
	node := &node[T]{value: &value}

	for {
		var (
			tail = load[T](&q.tail)
			next = load[T](&tail.next)
		)

		if tail == load[T](&q.tail) {
			if next == nil {
				if cas(&tail.next, next, node) {
					cas(&q.tail, tail, node)
					atomic.AddInt32(&q.queueSize, 1)

					return
				}
			} else {
				cas(&q.tail, tail, next)
			}
		}
	}
}

func (q *msQueue[T]) Dequeue() T {
	for {
		var (
			head = load[T](&q.head)
			tail = load[T](&q.tail)
			next = load[T](&head.next)
		)

		if head == load[T](&q.head) {
			if head == tail {
				if next == nil {
					return getZero[T]()
				}

				cas(&q.tail, tail, next)
			} else {
				value := *next.value
				if cas(&q.head, head, next) {
					atomic.AddInt32(&q.queueSize, -1)

					return value
				}
			}
		}
	}
}

func (q *msQueue[T]) IsEmpty() bool {
	return atomic.LoadInt32(&q.queueSize) == 0
}

func (q *msQueue[T]) Size() int32 {
	return atomic.LoadInt32(&q.queueSize)
}

func load[T any](p *unsafe.Pointer) *node[T] {
	return (*node[T])(atomic.LoadPointer(p))
}

func cas[T any](p *unsafe.Pointer, oldNode, newNode *node[T]) bool {
	return atomic.CompareAndSwapPointer(p, unsafe.Pointer(oldNode), unsafe.Pointer(newNode))
}

func getZero[T any]() T {
	var result T

	return result
}
