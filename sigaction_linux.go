// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build linux
// +build linux

package store

import (
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

type saflag uint64

const (
	_SA_RESTART saflag = 0x10000000
)

// NOTE: sigactiont does _not_ have the same layout as the C struct sigaction.
//
// It does, however, have the layout expected by the rt_sigaction system call,
// which is defined in asm-generic/signal.h.
type sigactiont struct {
	Handler  uintptr
	Flags    saflag
	Restorer uintptr

	// weirdly enough, even if the man page for rt_sigaction says it should
	// be sigset_t (of size 128), the kernel refuses 128 as a value in the
	// size param. libc says 8, though, and that works.
	Mask uint64
}

func sigaction(signum unix.Signal, act, old *sigactiont) error {
	_, _, errno := unix.RawSyscall6(unix.SYS_RT_SIGACTION,
		uintptr(signum),
		uintptr(unsafe.Pointer(act)),
		uintptr(unsafe.Pointer(old)),
		unsafe.Sizeof(act.Mask),
		0,
		0)

	runtime.KeepAlive(act)
	runtime.KeepAlive(old)
	if errno != 0 {
		return &os.SyscallError{Syscall: "rt_sigaction", Err: errno}
	}
	return nil
}

func gettid() int {
	return unix.Gettid()
}

func tgkill(pid, tid int, signal unix.Signal) error {
	return unix.Tgkill(pid, tid, signal)
}
