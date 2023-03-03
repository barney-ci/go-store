// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build unix
// +build unix

package store

import (
	"context"

	"golang.org/x/sys/unix"
)

var ErrWouldBlock = &likeError{Err: errWouldBlock, Like: unix.EWOULDBLOCK}

func lock(ctx context.Context, f OSFile, flags lockFlag) error {
	var sysFlags int
	if (flags & lockExcl) != 0 {
		sysFlags |= unix.LOCK_EX
	} else {
		sysFlags |= unix.LOCK_SH
	}
	if (flags & lockBlock) == 0 {
		sysFlags |= unix.LOCK_NB
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	for {
		err := unix.Flock(int(f.Fd()), sysFlags)
		switch {
		case err == nil:
			return nil
		case err == unix.EWOULDBLOCK:
			return wrapSyscallError("flock", ErrWouldBlock)
		case err == unix.EINTR:
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// This was a spurious EINTR wakeup. Retry the syscall.
			}
		default:
			return wrapSyscallError("flock", ErrWouldBlock)
		}
	}
}

func unlock(f OSFile) error {
	return wrapSyscallError("flock", unix.Flock(int(f.Fd()), unix.LOCK_UN))
}
