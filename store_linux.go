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
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lstatIno tries to use statx with STATX_INO (which is less IO demanding than
// regular stat), falling back to lstat/fstat if the syscall isn't implemented,
// for instance if the kernel is too old.
func lstatIno(dirfd int, path string) (uint64, error) {
	var statx unix.Statx_t
	err := unix.Statx(dirfd, path, unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_INO, &statx)
	switch {
	case err == nil:
		return statx.Ino, nil
	case errors.Is(err, unix.ENOSYS):
		// Fallback to Lstat or Fstat if ENOSYS
		var stat unix.Stat_t
		if path == "" {
			if err := unix.Fstat(dirfd, &stat); err != nil {
				return 0, &os.PathError{Op: "fstat", Path: fmt.Sprintf("fd:%d", dirfd), Err: err}
			}
		} else {
			if err := unix.Lstat(path, &stat); err != nil {
				return 0, &os.PathError{Op: "stat", Path: path, Err: err}
			}
		}
		return stat.Ino, nil
	default:
		name := path
		if name == "" {
			name = fmt.Sprintf("fd:%d", dirfd)
		}
		return 0, &os.PathError{Op: "statx", Path: name, Err: err}
	}
}

func deleted(f *os.File) (ok bool, e error) {
	fino, err := lstatIno(int(f.Fd()), "")
	if err != nil {
		return true, err
	}

	pino, err := lstatIno(unix.AT_FDCWD, f.Name())
	switch {
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	case err != nil:
		return true, err
	}
	return fino != pino, nil
}
