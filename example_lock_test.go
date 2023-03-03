// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store_test

import (
	"context"
	"log"
	"os"

	"barney.ci/go-store"
)

func init() {
	log.SetFlags(0)
}

func ExampleLock() {
	f, err := os.Open("/tmp")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Acquire an exclusive lock
	if err := store.Lock(context.Background(), f); err != nil {
		log.Fatal(err)
	}

	if err := store.Unlock(f); err != nil {
		log.Fatal(err)
	}

	// Closing the underlying file also releases the lock.
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

func ExampleLock_context() {

	// Open and exclusive-lock /tmp
	f, err := os.Open("/tmp")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := store.Lock(context.Background(), f); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Try to do the same in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)

		f, err := os.Open("/tmp")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		// This hangs until the context cancels
		err = store.Lock(ctx, f)
		if err != nil {
			log.Print(err)
		} else {
			log.Fatal("should not have succeeded locking")
		}
	}()

	// Cancel the context to abort the Lock operation
	cancel()
	<-done
}
