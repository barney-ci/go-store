// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store

import (
	"context"
	"errors"
	"os"
)

var errWouldBlock = errors.New("acquiring the lock would block")

// OSFile is an interface representing a file from which a file handle
// may be obtained. *os.File implements it.
type OSFile interface {
	Name() string
	Fd() uintptr
}

type lockFlag int

const (
	lockExcl lockFlag = 1 << iota
	lockBlock
)

// Lock acquires (or promotes an already acquired lock to) an exclusive lock,
// i.e. a lock used for writing, on the specified file.
//
// Lock is not re-entrant. Calling Lock on an exclusive lock is a no-op.
func Lock(ctx context.Context, f OSFile) error {
	return wrapPathError("exclusive lock", f.Name(), lock(ctx, f, lockExcl|lockBlock))
}

// RLock acquires (or demotes an already acquired lock to) a shared lock, i.e.
// a lock used for reading, on the specified file.
//
// RLock is not re-entrant. Calling RLock on a shared lock is a no-op.
func RLock(ctx context.Context, f OSFile) error {
	return wrapPathError("shared lock", f.Name(), lock(ctx, f, lockBlock))
}

// TryLock attempts to acquire (or promote an already acquired lock to) an exclusive lock,
// i.e. a lock used for writing, on the specified file.
//
// If the attempt would block, TryLock returns an error wrapping ErrWouldBlock.
func TryLock(f OSFile) error {
	return wrapPathError("exclusive lock (non-blocking)", f.Name(), lock(context.Background(), f, lockExcl))
}

// TryRLock attempts to acquire (or demote an already acquired lock to) a shared lock,
// i.e. a lock used for reading.
//
// If the attempt would block, TryRLock returns an error wrapping ErrWouldBlock.
func TryRLock(f OSFile) error {
	return wrapPathError("shared lock (non-blocking)", f.Name(), lock(context.Background(), f, 0))
}

// Unlock releases the lock on the specified file.
//
// Note that in almost all scenarios, closing the file is better. This is
// because if the underlying file handle has been duplicated (say, via dup(2)
// on Unix-like systems), then calling Unlock will release the underlying lock
// for _all_ of these file descriptors, whereas closing the file ensures
// that the lock gets released automatically once all file descriptors are
// closed.
func Unlock(f OSFile) error {
	return wrapPathError("tryrlock", f.Name(), unlock(f))
}

func wrapSyscallError(op string, err error) error {
	if err != nil {
		return &os.SyscallError{Syscall: op, Err: err}
	}
	return nil
}

func wrapPathError(op, path string, err error) error {
	if err != nil {
		return &os.PathError{Op: op, Path: path, Err: err}
	}
	return nil
}
