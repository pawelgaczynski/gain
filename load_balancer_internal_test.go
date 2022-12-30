package gain

import (
	"testing"

	. "github.com/stretchr/testify/require"
)

const numberOfTestWorkers = 4

type testWorker struct {
	conns int
}

func (w *testWorker) activeConnections() int {
	return w.conns
}

func (w *testWorker) setIndex(index int) {
}

func (w *testWorker) index() int {
	return 0
}

func (w *testWorker) loop(socket int) error {
	w.conns++
	return nil
}

func (w *testWorker) shutdown() {
}

func (w *testWorker) addConnToQueue(fd int) error {
	return nil
}

func (w *testWorker) ringFd() int {
	return 0
}

func (w *testWorker) started() bool {
	return true
}

func createTestWorkers() []*testWorker {
	workers := make([]*testWorker, 0)
	for i := 0; i < numberOfTestWorkers; i++ {
		workers = append(workers, &testWorker{})
	}
	return workers
}

//nolint:errcheck
func TestRoundRobinLoadBalander(t *testing.T) {
	lb := newRoundRobinLoadBalancer()
	workers := createTestWorkers()
	for _, worker := range workers {
		lb.register(worker)
	}
	worker := lb.next()
	worker.loop(0)
	Same(t, worker, workers[0])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[1])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[2])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[3])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[0])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[1])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[2])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[3])
}

//nolint:errcheck
func TestLeastConnectionsLoadBalander(t *testing.T) {
	lb := newLeastConnectionsLoadBalancer()
	workers := createTestWorkers()
	for _, worker := range workers {
		lb.register(worker)
	}
	workers[0].conns = 1
	workers[1].conns = 0
	workers[2].conns = 2
	workers[3].conns = 1
	worker := lb.next()
	worker.loop(0)
	Same(t, worker, workers[1])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[0])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[1])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[3])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[0])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[1])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[2])
	worker = lb.next()
	worker.loop(0)
	Same(t, worker, workers[3])
}
