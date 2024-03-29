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
func lstatIno(f *os.File, path string) (uint64, error) {
	dirfd := unix.AT_FDCWD
	if f != nil {
		dirfd = int(f.Fd())
	}

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

func openShared(path string, flag int, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, mode)
}

func rename(f OSFile, to string) error {
	return os.Rename(f.Name(), to)
}
