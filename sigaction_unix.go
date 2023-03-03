// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build unix && !linux
// +build unix,!linux

package store

import (
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

type saflag uint32

const (
	_SA_RESTART saflag = 0x2
)

func sigaction(signum unix.Signal, act, old *sigactiont) error {
	_, _, errno := unix.RawSyscall(unix.SYS_SIGACTION,
		uintptr(signum),
		uintptr(unsafe.Pointer(act)),
		uintptr(unsafe.Pointer(old)))

	runtime.KeepAlive(act)
	runtime.KeepAlive(old)
	if errno != 0 {
		return &os.SyscallError{Syscall: "sigaction", Err: errno}
	}
	return nil
}
