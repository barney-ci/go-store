// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build darwin
// +build darwin

package store

import (
	"golang.org/x/sys/unix"
)

var ErrWouldBlock = &likeError{Err: errWouldBlock, Like: unix.EWOULDBLOCK}

// Darwin doesn't seem to provide a way to interrupt a lock properly; even if
// were to send a Mach exception to the current Mach thread, this ends up not
// playing well with the Go runtime, which isn't expecting this.
const systemHasInterruptibleLocks = false

func preLock(f OSFile, flags lockFlag) {}

func lock(f OSFile, flags lockFlag) error {
	var sysFlags int
	if (flags & lockExcl) != 0 {
		sysFlags |= unix.LOCK_EX
	} else {
		sysFlags |= unix.LOCK_SH
	}
	if (flags & lockBlock) == 0 {
		sysFlags |= unix.LOCK_NB
	}

	err := unix.Flock(int(f.Fd()), sysFlags)
	switch {
	case err == nil:
		return nil
	case err == unix.EWOULDBLOCK:
		return wrapSyscallError("flock", ErrWouldBlock)
	default:
		return wrapSyscallError("flock", err)
	}
}

func unlock(f OSFile) error {
	return wrapSyscallError("flock", unix.Flock(int(f.Fd()), unix.LOCK_UN))
}

func lockGetThread() (any, error) {
	return nil, nil
}

func lockCloseThread(any) {}

func lockInterrupt(pidtid any) error {
	return nil
}
