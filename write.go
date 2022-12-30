package gain

import (
	"unsafe"

	"github.com/pawelgaczynski/gain/iouring"
)

type writer struct {
	ring *iouring.Ring
}

func (w *writer) addWriteRequest(conn *connection) (*iouring.SubmissionQueueEntry, error) {
	entry, err := w.ring.GetSQE()
	if err != nil {
		return nil, err
	}

	buffer := conn.buffer.data[:conn.index]

	entry.PrepareSend(conn.fd, uintptr(unsafe.Pointer(&buffer[0])), uint32(len(buffer)), 0)
	entry.UserData = writeDataFlag | uint64(conn.fd)

	conn.state = connWrite
	conn.setWriteKernel()

	return entry, nil
}

func newWriter(ring *iouring.Ring) *writer {
	return &writer{
		ring: ring,
	}
}
