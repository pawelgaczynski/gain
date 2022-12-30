package iouring

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

// Magic offsets for the application to mmap the data it needs.
const (
	offsqRing uint64 = 0
	offcqRing uint64 = 0x8000000
	offSQEs   uint64 = 0x10000000
)

func (ring *Ring) mmap(fileDescriptor int) error {
	ring.sqRing.ringSize = uint64(
		ring.params.sqOff.array,
	) + uint64(
		ring.params.sqEntries*(uint32)(unsafe.Sizeof(uint32(0))),
	)
	cqeSize := unsafe.Sizeof(CompletionQueueEvent{})
	ring.cqRing.ringSize = uint64(ring.params.cqOff.cqes) + uint64(ring.params.cqEntries*(uint32)(cqeSize))
	if ring.params.features&FeatSingleMMap > 0 {
		if ring.cqRing.ringSize > ring.sqRing.ringSize {
			ring.sqRing.ringSize = ring.cqRing.ringSize
		}
		ring.cqRing.ringSize = ring.sqRing.ringSize
	}
	data, err := syscall.Mmap(
		fileDescriptor,
		int64(offsqRing),
		int(ring.sqRing.ringSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
	)
	if err != nil {
		return err
	}
	ring.sqRing.buffer = data
	if ring.params.features&FeatSingleMMap > 0 {
		ring.cqRing.buffer = ring.sqRing.buffer
	} else {
		data, err = syscall.Mmap(
			fileDescriptor, int64(offcqRing), int(ring.cqRing.ringSize), syscall.PROT_READ|syscall.PROT_WRITE,
			syscall.MAP_SHARED|syscall.MAP_POPULATE)
		if err != nil {
			unmapErr := ring.UnmapRings()
			if unmapErr != nil {
				log.Panic(unmapErr)
			}
			return err
		}
		ring.cqRing.buffer = data
	}
	ringStart := &ring.sqRing.buffer[0]
	ring.sqRing.head = (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.head)))
	ring.sqRing.tail = (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.tail)))
	ring.sqRing.ringMask = (*uint32)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.ringMask)),
	)
	ring.sqRing.ringEntries = (*uint32)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.ringEntries)),
	)
	ring.sqRing.flags = (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.flags)))
	ring.sqRing.dropped = (*uint32)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.dropped)),
	)
	ring.sqRing.array = (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.sqOff.array)))
	sz := uintptr(ring.params.sqEntries) * unsafe.Sizeof(SubmissionQueueEntry{})
	buff, err := syscall.Mmap(
		fileDescriptor,
		int64(offSQEs),
		int(sz),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
	)
	if err != nil {
		unmapErr := ring.UnmapRings()
		if unmapErr != nil {
			log.Panic(unmapErr)
		}
		return err
	}
	ring.sqRing.sqeBuffer = buff
	cqRingPtr := uintptr(unsafe.Pointer(&ring.cqRing.buffer[0]))
	ringStart = &ring.cqRing.buffer[0]
	ring.cqRing.head = (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.cqOff.head)))
	ring.cqRing.tail = (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.cqOff.tail)))
	ring.cqRing.ringMask = (*uint32)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.cqOff.ringMask)),
	)
	ring.cqRing.ringEntries = (*uint32)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.cqOff.ringEntries)),
	)
	ring.cqRing.overflow = (*uint32)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.cqOff.overflow)),
	)
	ring.cqRing.cqeBuff = (*CompletionQueueEvent)(
		unsafe.Pointer(uintptr(unsafe.Pointer(ringStart)) + uintptr(ring.params.cqOff.cqes)),
	)
	if ring.params.cqOff.flags != 0 {
		ring.cqRing.flags = cqRingPtr + uintptr(ring.params.cqOff.flags)
	}
	return nil
}

func (ring *Ring) munmap() error {
	return syscall.Munmap(ring.sqRing.sqeBuffer)
}

func (ring *Ring) UnmapRings() error {
	var firstErr, secondErr error
	firstErr = syscall.Munmap(ring.sqRing.buffer)
	if ring.cqRing.buffer != nil && &ring.cqRing.buffer[0] != &ring.sqRing.buffer[0] {
		secondErr = syscall.Munmap(ring.cqRing.buffer)
	}
	if firstErr != nil || secondErr != nil {
		return fmt.Errorf("unmap first error: %s, second error: %s", firstErr.Error(), secondErr.Error()) // nolint: errorlint
	}
	return nil
}
