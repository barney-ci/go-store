// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
)

var (
	errWouldBlock      = errors.New("acquiring the lock would block")
	errLockInterrupted = errors.New("lock was interrupted; not a user-facing error, report a bug if you see this")
)

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
//
// NOTE: On Windows, Lock always releases any lock that was previously held
// when called. This means that callers must not assume that the lock is still
// held if Lock returns with an error.
func Lock(ctx context.Context, f OSFile) error {
	return wrapPathError("exclusive lock", f.Name(), interruptibleLock(ctx, f, lockExcl|lockBlock))
}

// RLock acquires (or demotes an already acquired lock to) a shared lock, i.e.
// a lock used for reading, on the specified file.
//
// RLock is not re-entrant. Calling RLock on a shared lock is a no-op.
//
// NOTE: On Windows, RLock always releases any lock that was previously held
// when called. This means that callers must not assume that the lock is still
// held if RLock returns with an error.
func RLock(ctx context.Context, f OSFile) error {
	return wrapPathError("shared lock", f.Name(), interruptibleLock(ctx, f, lockBlock))
}

// TryLock attempts to acquire (or promote an already acquired lock to) an exclusive lock,
// i.e. a lock used for writing, on the specified file.
//
// If the attempt would block, TryLock returns an error wrapping ErrWouldBlock.
//
// NOTE: On Windows, TryLock always releases any lock that was previously held
// when called. This means that callers must not assume that the lock is still
// held if TryLock returns with an error.
func TryLock(f OSFile) error {
	return wrapPathError("exclusive lock (non-blocking)", f.Name(), interruptibleLock(context.Background(), f, lockExcl))
}

// TryRLock attempts to acquire (or demote an already acquired lock to) a shared lock,
// i.e. a lock used for reading.
//
// If the attempt would block, TryRLock returns an error wrapping ErrWouldBlock.
//
// NOTE: On Windows, TryRLock always releases any lock that was previously held
// when called. This means that callers must not assume that the lock is still
// held if TryRLock returns with an error.
func TryRLock(f OSFile) error {
	return wrapPathError("shared lock (non-blocking)", f.Name(), interruptibleLock(context.Background(), f, 0))
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
	return wrapPathError("unlock", f.Name(), unlock(f))
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

func interruptibleLock(ctx context.Context, f OSFile, flags lockFlag) error {

	preLock(f, flags)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !systemHasInterruptibleLocks {
		return interruptibleLockFallback(ctx, f, flags)
	}

	if (flags & lockBlock) != 0 {
		// If this call is blocking, we have to do extra work to handle the cancellation case.

		// This chan gets closed on function return later on
		done := make(chan struct{})

		// This chan gets closed when the cancel goroutine is done
		canceldone := make(chan struct{})

		// We _must_ start this goroutine out of the LockOSThread block, otherwise
		// it'll just cancel itself in the go runtime, which panics
		cancelchan := make(chan func() error, 1)
		go func() {
			cancelfn := <-cancelchan
			defer close(canceldone)

			select {
			case <-done:
			case <-ctx.Done():
				// Double-check if we haven't already returned; we should only cancel
				// should only be called when we're actually blocking on a lock.
				select {
				case <-done:
					return
				default:
				}

				err := cancelfn()
				switch {
				case err != nil:
					panic(fmt.Errorf("Could not interrupt blocked lock call: %w", err))
				}
				return
			}
		}()

		// Force the goroutine to stay on the same thread; this is necessary because
		// we want to ensure the thread that executes the system call is the one
		// that ends up canceled/interrupted.
		runtime.LockOSThread()

		// This _must_ be deferred to ensure it runs even during a panic, not just
		// function return.
		defer runtime.UnlockOSThread()

		thread, err := lockGetThread()
		if err != nil {
			return err
		}
		defer lockCloseThread(thread)

		// Signal the cancel goroutine to no longer cancel the thread, and wait for it to
		// exit _before_ unlocking the OS thread.
		defer func() {
			close(done)
			<-canceldone
		}()

		cancelchan <- func() error {
			return lockInterrupt(thread)
		}
	}

	for {
		err := lock(f, flags)
		switch {
		case err == nil:
			return nil
		case err == errLockInterrupted:
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// This was a spurious wakeup. Retry the syscall.
			}
		default:
			return err
		}
	}
}

// interruptibleLockFallback falls back to a leaking goroutine approach
// on systems that do not support lock interrupts. This isn't great, of course,
// but allows the library to remain functional on these systems.
func interruptibleLockFallback(ctx context.Context, f OSFile, flags lockFlag) error {
	if (flags & lockBlock) == 0 {
		return lock(f, flags)
	}

	done := make(chan error, 1)
	go func() {
		done <- lock(f, flags)
	}()

	select {
	case <-ctx.Done():
		// If the goroutine finishes at the same time the context is done, we
		// want to give precedence to the goroutine error
		select {
		case err := <-done:
			return err
		default:
		}
		return ctx.Err()
	case err := <-done:
		return err
	}
}
