// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build unix && !linux
// +build unix,!linux

package store

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lstatIno(f *os.File, path string) (uint64, error) {
	var stat unix.Stat_t
	if path == "" {
		if err := unix.Fstat(int(f.Fd()), &stat); err != nil {
			return 0, &os.PathError{Op: "fstat", Path: fmt.Sprintf("fd:%d", int(f.Fd())), Err: err}
		}
	} else {
		if err := unix.Lstat(path, &stat); err != nil {
			return 0, &os.PathError{Op: "stat", Path: path, Err: err}
		}
	}
	return stat.Ino, nil
}

func openShared(path string, flag int, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, mode)
}

func rename(f OSFile, to string) error {
	return os.Rename(f.Name(), to)
}
