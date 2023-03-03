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
	"testing"
)

func TestStore(t *testing.T) {

	type Test struct {
		Example string
	}

	var val Test

	store := New[Test](json.NewEncoder, json.NewDecoder)
	dir := t.TempDir()

	// Test whether LoadStore correctly applies modifications
	t.Run("Modify", func(t *testing.T) {
		if err := store.Load(context.Background(), "testdata/example.json", &val); err != nil {
			t.Fatal(err)
		}
		if val.Example != "original" {
			t.Fatalf("expected original, got %v", val.Example)
		}

		if err := store.Store(context.Background(), filepath.Join(dir, "example.json"), 0777, &val); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "example.json")); err != nil {
			t.Fatal("expected Store to have created example.json, got error", err)
		}

		err := store.LoadStore(context.Background(), filepath.Join(dir, "example.json"), 0777, func(ctx context.Context, val *Test, _ error) error {
			val.Example = "modified"
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := store.Load(context.Background(), filepath.Join(dir, "example.json"), &val); err != nil {
			t.Fatal(err)
		}
		if val.Example != "modified" {
			t.Fatalf("expected modified, got %v", val.Example)
		}
	})

	// Test whether LoadStore works with load errors
	t.Run("NotExist", func(t *testing.T) {
		err := store.LoadStore(context.Background(), filepath.Join(dir, "missing.json"), 0777, func(ctx context.Context, val *Test, err error) error {
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
}
