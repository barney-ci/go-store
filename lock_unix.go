// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build unix
// +build unix

package store

import (
	"golang.org/x/sys/unix"
)

var ErrWouldBlock = &likeError{Err: errWouldBlock, Like: unix.EWOULDBLOCK}

const (
	// Picked to match Go's goroutine preemption signal.
	//
	// The reason for this is that we share the same rationale; see
	// https://cs.opensource.google/go/proposal/+/master:design/24543-non-cooperative-preemption.md
	// for the full context, quoting the relevant part:
	//
	//     **Choosing a signal.** We have to choose a signal that is unlikely to
	//     interfere with existing uses of signals or with debuggers.
	//     There are no perfect choices, but there are some heuristics.
	//
	//     1) It should be a signal that's passed-through by debuggers by
	//        default.
	//        On Linux, this is SIGALRM, SIGURG, SIGCHLD, SIGIO, SIGVTALRM, SIGPROF,
	//        and SIGWINCH, plus some glibc-internal signals.
	//     2) It shouldn't be used internally by libc in mixed Go/C binaries
	//        because libc may assume it's the only thing that can handle these
	//        signals.
	//        For example SIGCANCEL or SIGSETXID.
	//     3) It should be a signal that can happen spuriously without
	//        consequences.
	//        For example, SIGALRM is a bad choice because the signal handler can't
	//        tell if it was caused by the real process alarm or not (arguably this
	//        means the signal is broken, but I digress).
	//        SIGUSR1 and SIGUSR2 are also bad because those are often used in
	//        meaningful ways by applications.
	//     4) We need to deal with platforms without real-time signals (like
	//        macOS), so those are out.
	//
	// On the last note, it makes no difference to use SIGRT_N over SIGURG for
	// performance reasons -- the benchmarks end up the same.
	signo = unix.SIGURG
)

func init() {
	// Go installs its signal handler with SA_RESTART, which means we don't get
	// to handle EINTR; disable this for our signal, forever.
	//
	// While this seems we're breaking global state, because Go is expecting
	// all signal handlers to have SA_RESTART, the reality is that the Go authors
	// have to now explicitly make all of the stdlib code EINTR-resillient because
	// of CGo.
	//
	// Further readings:
	// * https://github.com/golang/go/issues/20400
	// * https://github.com/golang/go/issues/44761

	var act sigactiont
	if err := sigaction(signo, nil, &act); err != nil {
		panic(err)
	}
	act.Flags &= ^_SA_RESTART
	if err := sigaction(signo, &act, nil); err != nil {
		panic(err)
	}
}

func preLock(f OSFile, flags lockFlag) {}

func lock(f OSFile, flags lockFlag) error {
	var sysFlags int
	if (flags & lockExcl) != 0 {
		sysFlags |= unix.LOCK_EX
	} else {
		sysFlags |= unix.LOCK_SH
	}
	if (flags & lockBlock) == 0 {
		sysFlags |= unix.LOCK_NB
	}

	err := unix.Flock(int(f.Fd()), sysFlags)
	switch {
	case err == nil:
		return nil
	case err == unix.EINTR:
		return errLockInterrupted
	case err == unix.EWOULDBLOCK:
		return wrapSyscallError("flock", ErrWouldBlock)
	default:
		return wrapSyscallError("flock", err)
	}
}

func unlock(f OSFile) error {
	return wrapSyscallError("flock", unix.Flock(int(f.Fd()), unix.LOCK_UN))
}

func lockGetThread() (any, error) {
	pid := unix.Getpid()
	tid := gettid()
	return func() (int, int) { return pid, tid }, nil
}

func lockCloseThread(any) {}

func lockInterrupt(pidtid any) error {
	pid, tid := pidtid.(func() (int, int))()
	return tgkill(pid, tid, signo)
}
