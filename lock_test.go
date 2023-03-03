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
	"path/filepath"
	"testing"
)

func makeLockfiles(tb testing.TB, path string, n int) chan *os.File {
	tb.Helper()

	locks := make(chan *os.File, 32)
	go func() {
		for i := 0; i < n; i++ {
			f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0777)
			if err != nil {
				tb.Error(err)
			}
			locks <- f
		}
	}()
	return locks
}

func TestLock(t *testing.T) {

	t.Run("Lock", func(t *testing.T) {
		t.Parallel()

		locks := makeLockfiles(t, filepath.Join(t.TempDir(), "barney-ci-go-store-lock-test-1"), 2)

		f1 := <-locks
		if f1 == nil {
			t.FailNow()
		}
		defer f1.Close()

		f2 := <-locks
		if f2 == nil {
			t.FailNow()
		}
		defer f2.Close()

		// Write-locking a second fd of a file that was exclusive-locked should block
		if err := Lock(context.Background(), f1); err != nil {
			t.Fatal(err)
		}
		if err := TryLock(f2); err == nil {
			t.Fatalf("TryLock succeeded on an acquired lock")
		}
		if err := Unlock(f1); err != nil {
			t.Fatal(err)
		}
		if err := TryLock(f2); err != nil {
			t.Fatal(err)
		}
		if err := Unlock(f2); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("RLock", func(t *testing.T) {
		t.Parallel()

		locks := makeLockfiles(t, filepath.Join(t.TempDir(), "barney-ci-go-store-rlock-test"), 2)

		f1 := <-locks
		if f1 == nil {
			t.FailNow()
		}
		defer f1.Close()

		f2 := <-locks
		if f2 == nil {
			t.FailNow()
		}
		defer f2.Close()

		// Read-locking multiple fds of the same file should not block
		if err := TryRLock(f1); err != nil {
			t.Fatal(err)
		}
		if err := TryRLock(f2); err != nil {
			t.Fatal(err)
		}

		// Promoting a shared lock to an exclusive lock when another shared lock is held should block
		if err := TryLock(f2); !errors.Is(err, ErrWouldBlock) {
			t.Fatalf("TryRLock failed with error other than ErrWouldBlock: %T %v", err, err)
		}

		// Promoting a shared lock when only one fd holds it should work
		if err := Unlock(f1); err != nil {
			t.Fatal(err)
		}
		if err := TryLock(f2); err != nil {
			t.Fatal(err)
		}
		if err := TryLock(f1); !errors.Is(err, ErrWouldBlock) {
			t.Fatalf("TryRLock failed with error other than ErrWouldBlock: %T %v", err, err)
		}

		// Demoting an exclusive lock to a shared lock should work
		if err := TryRLock(f2); err != nil {
			t.Fatal(err)
		}
		if err := TryRLock(f1); err != nil {
			t.Fatal(err)
		}
	})

}

func BenchmarkLock(b *testing.B) {

	var lockpath = filepath.Join(b.TempDir(), "barney-ci-go-store-lock-bench")

	b.Run("Sequential", func(b *testing.B) {
		var count int

		b.StopTimer()
		locks := makeLockfiles(b, lockpath, b.N)
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			f := <-locks
			if f == nil {
				b.FailNow()
			}
			Lock(context.Background(), f)
			count++
			f.Close()
		}

		b.StopTimer()
		if count != b.N {
			b.Fatalf("expected %d increments, got %d", b.N, count)
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		var count int

		b.StopTimer()
		locks := makeLockfiles(b, lockpath, b.N)
		b.StartTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				f := <-locks
				if f == nil {
					b.FailNow()
				}
				if err := Lock(context.Background(), f); err != nil {
					b.Fatal(err)
				}
				count++
				f.Close()
			}
		})

		b.StopTimer()
		if count != b.N {
			b.Fatalf("expected %d increments, got %d", b.N, count)
		}
	})
}
