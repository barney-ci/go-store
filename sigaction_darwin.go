// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build darwin
// +build darwin

package store

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type sigactiont struct {
	Handler uintptr
	Tramp   uintptr
	Mask    uint32
	Flags   saflag
}

func gettid() int {
	// Darwin doesn't have a Gettid wrapper, but the syscall exists.
	tid, _, errno := unix.RawSyscall(unix.SYS_GETTID, 0, 0, 0)
	if errno != 0 {
		panic(fmt.Sprintf("gettid(2) should always succeed; got errno %d: %v", errno, errno))
	}
	return int(tid)
}

// On macOS, tgkill does not exist, and this wrapper simply calls kill(tid, signal).
//
// The reason why Linux needs tgkill and Darwin doesn't is because Linux will
// aggressively reuse thread IDs among processes, while Darwin is more conservative.
func tgkill(_, tid int, signal unix.Signal) error {
	return unix.Kill(tid, unix.Signal(signal))
}
