// Copyright 2023 Paweł Gaczyński.
// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Original source: https://github.com/golang-design/lockfree/blob/master/stack.go

package stack

import (
	"sync/atomic"
	"unsafe"
)

type node[T any] struct {
	value T
	next  unsafe.Pointer
}

type Stack[T any] struct {
	top unsafe.Pointer
	len uint64
}

// NewStack creates a new lock-free queue.
func NewLockFreeStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

func getZero[T any]() T {
	var result T

	return result
}

// Pop pops value from the top of the stack.
func (s *Stack[T]) Pop() T {
	var (
		top, next unsafe.Pointer
		item      *node[T]
	)

	for {
		top = atomic.LoadPointer(&s.top)
		if top == nil {
			return getZero[T]()
		}
		item = (*node[T])(top)
		next = atomic.LoadPointer(&item.next)

		if atomic.CompareAndSwapPointer(&s.top, top, next) {
			atomic.AddUint64(&s.len, ^uint64(0))

			return item.value
		}
	}
}

// Push pushes a value on top of the stack.
func (s *Stack[T]) Push(v T) {
	var (
		item = node[T]{value: v}
		top  unsafe.Pointer
	)

	for {
		top = atomic.LoadPointer(&s.top)
		item.next = top

		if atomic.CompareAndSwapPointer(&s.top, top, unsafe.Pointer(&item)) {
			atomic.AddUint64(&s.len, 1)

			return
		}
	}
}
