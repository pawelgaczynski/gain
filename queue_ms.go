// nolint: unused
package gain

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

func newIntQueue() lockFreeQueue[int] {
	n := unsafe.Pointer(&node[int]{})
	return &msQueue[int]{head: n, tail: n}
}

func newConnectionQueue() lockFreeQueue[*connection] {
	n := unsafe.Pointer(&node[*connection]{})
	return &msQueue[*connection]{head: n, tail: n}
}

func (q *msQueue[T]) enqueue(value T) {
	node := &node[T]{value: &value}
	for {
		tail := load[T](&q.tail)
		next := load[T](&tail.next)
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

func (q *msQueue[T]) dequeue() T {
	for {
		head := load[T](&q.head)
		tail := load[T](&q.tail)
		next := load[T](&head.next)
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

func (q *msQueue[T]) isEmpty() bool {
	return atomic.LoadInt32(&q.queueSize) == 0
}

func (q *msQueue[T]) size() int32 {
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
