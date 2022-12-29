package iouring_test

import (
	"testing"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/stretchr/testify/assert"
)

func TestPrepareMsgRing(t *testing.T) {
	entry := &iouring.SubmissionQueueEntry{}
	entry.PrepareMsgRing(10, 100, 200, 60)

	assert.Equal(t, uint8(40), entry.OpCode)
	assert.Equal(t, int32(10), entry.Fd)
	assert.Equal(t, uint32(100), entry.Len)
	assert.Equal(t, uint64(200), entry.Off)
	assert.Equal(t, uint32(60), entry.OpcodeFlags)
}

func TestPrepareAccept(t *testing.T) {
	entry := &iouring.SubmissionQueueEntry{}
	entry.PrepareAccept(10, 100, 200, 60)

	assert.Equal(t, uint8(13), entry.OpCode)
	assert.Equal(t, int32(10), entry.Fd)
	assert.Equal(t, uint64(100), entry.Addr)
	assert.Equal(t, uint64(200), entry.Off)
	assert.Equal(t, uint32(60), entry.OpcodeFlags)
}

func TestPrepareClose(t *testing.T) {
	entry := &iouring.SubmissionQueueEntry{}
	entry.PrepareClose(10)

	assert.Equal(t, uint8(19), entry.OpCode)
	assert.Equal(t, int32(10), entry.Fd)
}
