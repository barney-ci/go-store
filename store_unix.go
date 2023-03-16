// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build unix && !linux
// +build unix,!linux

package store

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func deleted(f *os.File) (bool, error) {
	var fstat, pathstat unix.Stat_t

	if err := unix.Fstat(int(f.Fd()), &fstat); err != nil {
		return true, &os.PathError{Op: "fstat", Path: "<fd>", Err: err}
	}
	err := unix.Lstat(f.Name(), &pathstat)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	case err != nil:
		return true, &os.PathError{Op: "stat", Path: f.Name(), Err: err}
	}
	return fstat.Ino != pathstat.Ino, nil
}

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
