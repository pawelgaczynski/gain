package gain

import (
	"unsafe"

	"github.com/pawelgaczynski/gain/iouring"
)

type reader struct {
	ring *iouring.Ring
}

func (r *reader) addReadRequest(conn *connection) (*iouring.SubmissionQueueEntry, error) {
	entry, err := r.ring.GetSQE()
	if err != nil {
		return nil, err
	}
	buffer := conn.buffer.data

	entry.PrepareRecv(conn.fd, uintptr(unsafe.Pointer(&buffer[0])), uint32(len(buffer)), 0)
	entry.UserData = readDataFlag | uint64(conn.fd)

	conn.state = connRead
	conn.setReadKernel()

	return entry, nil
}

func newReader(ring *iouring.Ring) *reader {
	return &reader{
		ring: ring,
	}
}
