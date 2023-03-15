// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStore(t *testing.T) {

	type Test struct {
		Example string
	}

	var val Test

	store := New[Test](json.NewEncoder, json.NewDecoder)
	dir := t.TempDir()

	// Test whether LoadAndStore correctly applies modifications
	t.Run("Modify", func(t *testing.T) {
		if _, err := store.Load(context.Background(), "testdata/example.json", &val); err != nil {
			t.Fatal(err)
		}
		if val.Example != "original" {
			t.Fatalf("expected original, got %v", val.Example)
		}

		if err := store.Store(context.Background(), filepath.Join(dir, "example.json"), 0777, &val, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "example.json")); err != nil {
			t.Fatal("expected Store to have created example.json, got error", err)
		}

		err := store.LoadAndStore(context.Background(), filepath.Join(dir, "example.json"), 0777, func(ctx context.Context, val *Test, _ error) error {
			val.Example = "modified"
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := store.Load(context.Background(), filepath.Join(dir, "example.json"), &val); err != nil {
			t.Fatal(err)
		}
		if val.Example != "modified" {
			t.Fatalf("expected modified, got %v", val.Example)
		}
	})

	// Test whether LoadAndStore works with load errors
	t.Run("NotExist", func(t *testing.T) {
		err := store.LoadAndStore(context.Background(), filepath.Join(dir, "missing.json"), 0777, func(ctx context.Context, val *Test, err error) error {
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("error must be ErrNotExist, got %q instead", err)
			}
			if val == nil {
				t.Fatal("val must never be nil")
			}
			if *val != (Test{}) {
				t.Fatal("val must be the zero value")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Stress", func(t *testing.T) {
		store := New[int](json.NewEncoder, json.NewDecoder)

		const total = 1000

		var wait sync.WaitGroup
		for i := 0; i < total; i++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				err := store.LoadAndStore(context.Background(), filepath.Join(dir, "num"), 0777, func(ctx context.Context, val *int, err error) error {
					*val++
					return nil
				})
				if err != nil {
					t.Error(err)
				}
			}()
		}
		wait.Wait()

		var num int
		if _, err := store.Load(context.Background(), filepath.Join(dir, "num"), &num); err != nil {
			t.Fatal(err)
		}
		if num != total {
			t.Fatalf("expected total to be %d, got %d", total, num)
		}
	})
}

func TestRename(t *testing.T) {
	// Ensure rename() works correctly on all platforms

	// The process should have an open file handle on the destination
	dir := t.TempDir()
	f0, err := openShared(filepath.Join(dir, "new"), os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f0.Close()

	// Open the original file, then rename it to the destination
	f, err := openShared(filepath.Join(dir, "orig"), os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := rename(f, filepath.Join(dir, "new")); err != nil {
		t.Fatal(err)
	}
	f.Close()
}
