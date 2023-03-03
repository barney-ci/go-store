// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build linux
// +build linux

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func deleted(f *os.File) (bool, error) {
	var fstat, pathstat unix.Statx_t

	if err := unix.Statx(int(f.Fd()), "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_INO, &fstat); err != nil {
		return true, &os.PathError{Op: "statx", Path: "fd:" + f.Name(), Err: err}
	}
	err := unix.Statx(unix.AT_FDCWD, f.Name(), unix.AT_SYMLINK_NOFOLLOW, unix.STATX_INO, &pathstat)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	case err != nil:
		return true, &os.PathError{Op: "statx", Path: f.Name(), Err: err}
	}
	return fstat.Ino != pathstat.Ino, nil
}
