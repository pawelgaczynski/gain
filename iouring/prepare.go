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

package iouring

import (
	"log"
	"syscall"
	"time"
	"unsafe"
)

func (entry *SubmissionQueueEntry) setTargetFixedFile(fileIndex uint) {
	entry.SpliceFdIn = int32(fileIndex + 1)
}

func (entry *SubmissionQueueEntry) prepareRW(opcode uint8, fd int, addr uintptr, length uint32, offset uint64) {
	entry.OpCode = opcode
	entry.Flags = 0
	entry.IoPrio = 0
	entry.Fd = int32(fd)
	entry.Off = offset
	entry.Addr = uint64(addr)
	entry.Len = length
	entry.UserData = 0
	entry.BufIG = 0
	entry.Personality = 0
	entry.SpliceFdIn = 0
	entry._pad2[0] = 0
	entry._pad2[1] = 0
}

func (entry *SubmissionQueueEntry) PrepareSplice(
	fdIn int, offIn int64, fdOut int, offOut int64, nbytes uint, spliceFlags uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareTee(fdIn int, fdOut int, nbytes uint, spliceFlags uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareReadv(fd int, iovecs uintptr, nrVecs uint32, offset uint64) {
	entry.prepareRW(OpReadv, fd, iovecs, nrVecs, offset)
}

func (entry *SubmissionQueueEntry) PrepareReadv2(fd int, iovecs uintptr, nrVecs uint32, offset uint64, flags uint32) {
	entry.PrepareReadv(fd, iovecs, nrVecs, offset)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareReadFixed(
	fileDescriptor int,
	vectors uintptr,
	length uint32,
	offset uint64,
	index int,
) {
	entry.prepareRW(OpReadFixed, fileDescriptor, vectors, length, offset)
	entry.BufIG = uint16(index)
}

func (entry *SubmissionQueueEntry) PrepareWritev(
	fd int,
	iovecs uintptr,
	nrVecs uint32,
	offset uint64,
) {
	entry.prepareRW(OpWritev, fd, iovecs, nrVecs, offset)
}

func (entry *SubmissionQueueEntry) PrepareWritev2(
	fd int,
	iovecs uintptr,
	nrVecs uint32,
	offset uint64,
	flags uint32,
) {
	entry.PrepareWritev(fd, iovecs, nrVecs, offset)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareWriteFixed(
	fileDescriptor int,
	vectors uintptr,
	length uint32,
	offset uint64,
	index int,
) {
	entry.prepareRW(OpWriteFixed, fileDescriptor, vectors, length, offset)
	entry.BufIG = uint16(index)
}

func (entry *SubmissionQueueEntry) PrepareRecvMsg(
	fd int,
	msg uintptr,
	flags uint32,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSendMsg(
	fd int,
	msg uintptr,
	flags uint32,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PreparePollAdd(fd int, pollMask uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PreparePollMultishot(fd int, pollMask uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PreparePollRemove(fd int, userData uint64) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PreparePollUpdate(fd int, oldUserData, newUserData uint64, pollMask, flags uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareFsync(fd int, fsyncFlags uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareNop() {
	entry.prepareRW(OpNop, -1, 0, 0, 0)
}

func (entry *SubmissionQueueEntry) PrepareTimeout(duration time.Duration, count uint64, flags uint32) {
	spec := syscall.NsecToTimespec(duration.Nanoseconds())
	entry.prepareRW(OpTimeout, -1, uintptr(unsafe.Pointer(&spec)), 1, count)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareTimeoutRemove(duration time.Duration, count uint64, flags uint32) {
	spec := syscall.NsecToTimespec(duration.Nanoseconds())
	entry.prepareRW(OpTimeoutRemove, -1, uintptr(unsafe.Pointer(&spec)), 1, count)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareTimeoutUpdate(duration time.Duration, count uint64, flags uint32) {
	spec := syscall.NsecToTimespec(duration.Nanoseconds())
	entry.prepareRW(OpTimeoutRemove, -1, uintptr(unsafe.Pointer(&spec)), 1, count)
	entry.OpcodeFlags = flags | TimeoutUpdate
}

func (entry *SubmissionQueueEntry) PrepareAccept(fd int, addr uintptr, addrLen uint64, flags uint32) {
	entry.prepareRW(OpAccept, fd, addr, 0, addrLen)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareAcceptDirect(
	fd int, addr uintptr, addrLen uint64, flags uint32, fileIndex uint) {
	entry.PrepareAccept(fd, addr, addrLen, flags)
	entry.setTargetFixedFile(fileIndex)
}

func (entry *SubmissionQueueEntry) PrepareCancel(userData uint64, flags int) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareLinkTimeout(ts *syscall.Timespec, flags uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareConnect(fd int, addr uintptr, addrLen uint64) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareConnectFilesUpdate(fds []int, nrFds uint64, offset uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareFallocate(fd int, mode int, offset uint64, length uint64) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareOpenat(dfd int, path string, flags int, mode uint32) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareOpenatDirect(dfd int, path string, flags int, mode uint32, fileIndex uint) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareClose(fd int) {
	entry.prepareRW(OpClose, fd, 0, 0, 0)
}

func (entry *SubmissionQueueEntry) PrepareCloseDirect(fileIndex uint) {
	entry.PrepareClose(0)
	entry.setTargetFixedFile(fileIndex)
}

func (entry *SubmissionQueueEntry) PrepareRead(
	fd int,
	buffer uintptr,
	nbytes uint32,
	offset uint64,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareWrite(
	fd int,
	buffer uintptr,
	nbytes uint32,
	offset uint64,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareStatx(
	dfd int,
	path string,
	flags int,
	mask uint,
	statxbuf uintptr,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareFadvise(
	fd int,
	offset uint64,
	length uint32,
	advise int,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareMadvise(
	addr uintptr,
	length uint32,
	advise int,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSend(
	fileDescriptor int,
	addr uintptr,
	length uint32,
	flags uint32,
) {
	entry.prepareRW(OpSend, fileDescriptor, addr, length, 0)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareSendZC(
	fileDescriptor int,
	addr uintptr,
	length uint32,
	flags uint32,
	zcFlags uint,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSendSetAddr(
	addr uintptr,
	length uint16,
) {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareRecv(
	fileDescriptor int,
	addr uintptr,
	length uint32,
	flags uint32,
) {
	entry.prepareRW(OpRecv, fileDescriptor, addr, length, 0)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareRecvMultishot(
	fileDescriptor int,
	addr uintptr,
	length uint32,
	flags uint32,
) {
	entry.PrepareRecv(fileDescriptor, addr, length, flags)
	entry.IoPrio |= RecvMultishot
}

func (entry *SubmissionQueueEntry) PrepareOpenat2() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareOpenat2Direct() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareEpollCtrl() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareProvideBuffers(
	addr uintptr,
	length uint32,
	fileDescriptor int,
	bgid uint64,
	off uint64,
) {
	entry.OpCode = OpProvideBuffers
	entry.Flags = 0
	entry.IoPrio = 0
	entry.Fd = int32(fileDescriptor)
	entry.Off = off
	entry.Addr = uint64(addr)
	entry.Len = length
	entry.OpcodeFlags = 0
	entry.UserData = 0
	entry.BufIG = uint16(bgid)
	entry.Personality = 0
	entry.SpliceFdIn = 0
	entry._pad2[0] = 0
	entry._pad2[1] = 0
}

func (entry *SubmissionQueueEntry) PrepareRemoveBuffers() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareShutdown() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareUnlinkat() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareUnlink() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareRenameat() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareRename() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSyncFileRange() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareMkdirat() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareMkdir() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSymlinkat() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSymlink() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareLinkat() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareLink() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareMsgRing(fd int, length uint32, data uint64, flags uint32) {
	entry.prepareRW(OpMsgRing, fd, 0, length, data)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareGetxattr() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSetxattr() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareFgetxattr() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareFsetxattr() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSocket() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSocketDirect() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareSocketDirectAlloc() {
	// TODO
	log.Fatal(errNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareTimeout2(ts *syscall.Timespec, count uint64, flags uint32) {
	entry.prepareRW(OpTimeout, -1, uintptr(unsafe.Pointer(ts)), 1, count)
	entry.OpcodeFlags = flags
}

func (entry *SubmissionQueueEntry) PrepareUpdateTimeout2(ts *syscall.Timespec, count uint64, flags uint32) {
	entry.prepareRW(OpTimeoutRemove, -1, uintptr(unsafe.Pointer(ts)), 1, count)
	entry.OpcodeFlags = flags | TimeoutUpdate
}

func (entry *SubmissionQueueEntry) PrepareRemoveTimeout2(ts *syscall.Timespec, count uint64, flags uint32) {
	entry.prepareRW(OpTimeoutRemove, -1, uintptr(unsafe.Pointer(ts)), 1, count)
	entry.OpcodeFlags = flags
}
