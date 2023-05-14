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

// PrepareSplice has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSplice(
	_ int, _ int64, _ int, _ int64, _ uint, _ uint,
) {
	log.Panic(ErrNotImplemented)
}

// PrepareTee has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareTee(_ int, _ int, _ uint, _ uint) {
	log.Panic(ErrNotImplemented)
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

func (entry *SubmissionQueueEntry) PrepareSendMsg(
	fileDescriptor int,
	msg *syscall.Msghdr,
	flags uint32,
) {
	entry.prepareRW(OpSendmsg, fileDescriptor, uintptr(unsafe.Pointer(msg)), 1, 0)
	entry.OpcodeFlags = flags
}

// PreparePollAdd has not been implemented yet.
func (entry *SubmissionQueueEntry) PreparePollAdd(_ int, _ uint) {
	log.Panic(ErrNotImplemented)
}

// PreparePollMultishot has not been implemented yet.
func (entry *SubmissionQueueEntry) PreparePollMultishot(_ int, _ uint) {
	log.Panic(ErrNotImplemented)
}

// PreparePollRemove has not been implemented yet.
func (entry *SubmissionQueueEntry) PreparePollRemove(_ int, _ uint64) {
	log.Panic(ErrNotImplemented)
}

// PreparePollUpdate has not been implemented yet.
func (entry *SubmissionQueueEntry) PreparePollUpdate(_ int, _, _ uint64, _, _ uint) {
	log.Panic(ErrNotImplemented)
}

// PrepareFsync has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareFsync(_ int, _ uint) {
	log.Panic(ErrNotImplemented)
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
	fd int, addr uintptr, addrLen uint64, flags uint32, fileIndex uint,
) {
	entry.PrepareAccept(fd, addr, addrLen, flags)
	entry.setTargetFixedFile(fileIndex)
}

// PrepareCancel has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareCancel(_ uint64, _ int) {
	log.Panic(ErrNotImplemented)
}

// PrepareLinkTimeout has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareLinkTimeout(_ *syscall.Timespec, _ uint) {
	log.Panic(ErrNotImplemented)
}

// PrepareConnect has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareConnect(_ int, _ uintptr, _ uint64) {
	log.Panic(ErrNotImplemented)
}

// PrepareConnectFilesUpdate has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareConnectFilesUpdate(_ []int, _ uint64, _ uint) {
	log.Panic(ErrNotImplemented)
}

// PrepareFallocate has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareFallocate(_ int, _ int, _ uint64, _ uint64) {
	log.Panic(ErrNotImplemented)
}

// PrepareOpenat has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareOpenat(_ int, _ string, _ int, _ uint32) {
	log.Panic(ErrNotImplemented)
}

// PrepareOpenatDirect has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareOpenatDirect(_ int, _ string, _ int, _ uint32, _ uint) {
	log.Panic(ErrNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareClose(fd int) {
	entry.prepareRW(OpClose, fd, 0, 0, 0)
}

func (entry *SubmissionQueueEntry) PrepareCloseDirect(fileIndex uint) {
	entry.PrepareClose(0)
	entry.setTargetFixedFile(fileIndex)
}

// PrepareRead has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareRead(
	_ int,
	_ uintptr,
	_ uint32,
	_ uint64,
) {
	log.Panic(ErrNotImplemented)
}

// PrepareWrite has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareWrite(
	_ int,
	_ uintptr,
	_ uint32,
	_ uint64,
) {
	log.Panic(ErrNotImplemented)
}

// PrepareStatx has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareStatx(
	_ int,
	_ string,
	_ int,
	_ uint,
	_ uintptr,
) {
	log.Panic(ErrNotImplemented)
}

// PrepareFadvise has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareFadvise(
	_ int,
	_ uint64,
	_ uint32,
	_ int,
) {
	log.Panic(ErrNotImplemented)
}

// PrepareMadvise has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareMadvise(
	_ uintptr,
	_ uint32,
	_ int,
) {
	log.Panic(ErrNotImplemented)
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

// PrepareSendZC has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSendZC(
	_ int,
	_ uintptr,
	_ uint32,
	_ uint32,
	_ uint,
) {
	log.Panic(ErrNotImplemented)
}

// PrepareSendSetAddr has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSendSetAddr(
	_ uintptr,
	_ uint16,
) {
	log.Panic(ErrNotImplemented)
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

func (entry *SubmissionQueueEntry) PrepareRecvMsg(
	fileDescriptor int,
	msg *syscall.Msghdr,
	flags uint32,
) {
	entry.prepareRW(OpRecvmsg, fileDescriptor, uintptr(unsafe.Pointer(msg)), 1, 0)
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

// PrepareOpenat2 has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareOpenat2() {
	log.Panic(ErrNotImplemented)
}

// PrepareOpenat2Direct has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareOpenat2Direct() {
	log.Panic(ErrNotImplemented)
}

// PrepareEpollCtrl has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareEpollCtrl() {
	log.Panic(ErrNotImplemented)
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

// PrepareRemoveBuffers has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareRemoveBuffers() {
	log.Panic(ErrNotImplemented)
}

// PrepareShutdown has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareShutdown() {
	log.Panic(ErrNotImplemented)
}

// PrepareUnlinkat has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareUnlinkat() {
	log.Panic(ErrNotImplemented)
}

// PrepareUnlink has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareUnlink() {
	log.Panic(ErrNotImplemented)
}

// PrepareRenameat has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareRenameat() {
	log.Panic(ErrNotImplemented)
}

// PrepareRename has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareRename() {
	log.Panic(ErrNotImplemented)
}

// PrepareSyncFileRange has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSyncFileRange() {
	log.Panic(ErrNotImplemented)
}

// PrepareMkdirat has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareMkdirat() {
	log.Panic(ErrNotImplemented)
}

// PrepareMkdir has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareMkdir() {
	log.Panic(ErrNotImplemented)
}

// PrepareSymlinkat has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSymlinkat() {
	log.Panic(ErrNotImplemented)
}

// PrepareSymlink has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSymlink() {
	log.Panic(ErrNotImplemented)
}

// PrepareLinkat has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareLinkat() {
	log.Panic(ErrNotImplemented)
}

// PrepareLink has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareLink() {
	log.Panic(ErrNotImplemented)
}

func (entry *SubmissionQueueEntry) PrepareMsgRing(fd int, length uint32, data uint64, flags uint32) {
	entry.prepareRW(OpMsgRing, fd, 0, length, data)
	entry.OpcodeFlags = flags
}

// PrepareGetxattr has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareGetxattr() {
	log.Panic(ErrNotImplemented)
}

// PrepareSetxattr has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSetxattr() {
	log.Panic(ErrNotImplemented)
}

// PrepareFgetxattr has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareFgetxattr() {
	log.Panic(ErrNotImplemented)
}

// PrepareFsetxattr has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareFsetxattr() {
	log.Panic(ErrNotImplemented)
}

// PrepareSocket has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSocket() {
	log.Panic(ErrNotImplemented)
}

// PrepareSocketDirect has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSocketDirect() {
	log.Panic(ErrNotImplemented)
}

// PrepareSocketDirectAlloc has not been implemented yet.
func (entry *SubmissionQueueEntry) PrepareSocketDirectAlloc() {
	log.Panic(ErrNotImplemented)
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
