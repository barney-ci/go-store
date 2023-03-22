// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build windows
// +build windows

package store

import (
	"syscall"

	"golang.org/x/sys/windows"
)

var ErrWouldBlock = errWouldBlock

var procCancelSynchronousIo = windows.MustLoadDLL("kernel32.dll").MustFindProc("CancelSynchronousIo")

const systemHasInterruptibleLocks = true

func cancelSynchronousIo(h windows.Handle) error {
	r1, _, e1 := syscall.SyscallN(procCancelSynchronousIo.Addr(), uintptr(h))
	if r1 == 0 {
		return wrapSyscallError("CancelSynchronousIo", e1)
	}
	return nil
}

func preLock(f OSFile, flags lockFlag) {
	// The lock promotion and demotion logic is a bit weird. On windows, a handle may
	// hold both a shared and an exclusive lock on the same file handle, and the handle has
	// to be unlocked _twice_: the first call unlocks the exclusive lock, and the second the
	// shared lock. Since we can't query the lock state, rather than performing some locking
	// operations that leave us in the same state regardless of whether a shared/exclusive
	// lock is currently held, we simply always unlock prior any operation.
	//
	// NOTE: it does mean that on windows, locking and cancelling the context will release the
	// lock, and Try(R)Lock will release the lock even when it errors out. Too bad!

	_ = unlock(f)
}

func lock(f OSFile, flags lockFlag) error {
	var sysFlags uint32
	if (flags & lockExcl) != 0 {
		sysFlags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	if (flags & lockBlock) == 0 {
		sysFlags |= windows.LOCKFILE_FAIL_IMMEDIATELY
	}

	var overlapped windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), sysFlags, 0, ^uint32(0), ^uint32(0), &overlapped)
	switch {
	case err == nil:
		return nil
	case err == windows.ERROR_OPERATION_ABORTED:
		return errLockInterrupted
	case err == windows.ERROR_LOCK_VIOLATION && (flags&lockBlock) == 0:
		return wrapSyscallError("LockFileEx", ErrWouldBlock)
	default:
		return wrapSyscallError("LockFileEx", err)
	}
}

func unlock(f OSFile) error {
	var overlapped windows.Overlapped
	return wrapSyscallError("UnlockFileEx", windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), &overlapped))
}

func lockGetThread() (any, error) {
	var thread windows.Handle
	err := windows.DuplicateHandle(
		windows.CurrentProcess(),
		windows.CurrentThread(),
		windows.CurrentProcess(),
		&thread,
		0, false, windows.DUPLICATE_SAME_ACCESS)
	if err != nil {
		return nil, wrapSyscallError("DuplicateHandle", err)
	}
	return thread, nil
}

func lockCloseThread(thread any) {
	windows.CloseHandle(thread.(windows.Handle))
}

func lockInterrupt(thread any) error {
	return cancelSynchronousIo(thread.(windows.Handle))
}
