package gain

type lockFreeQueue[T any] interface {
	enqueue(T)
	dequeue() T
	isEmpty() bool
	size() int32
}
