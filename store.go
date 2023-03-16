// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store

import (
	"context"
	"errors"
	"io"
	"os"
)

var ErrRetry = errors.New("the operation needs to be retried")

type Decoder interface {
	Decode(v any) error
}

type Encoder interface {
	Encode(v any) error
}

// A Store represents a way to marshal and unmarshal values of type T atomically
// from and to the file system.
//
// Basic usage is:
//
//	st := store.New[Type](json.NewEncoder, json.NewDecoder)
//
//	err := st.LoadAndStore(context.Background(), "/path/to/state.json", 0666, func(val *Type) error {
//	    // Use and/or modify val; it will get re-marshaled to the file
//	    return nil
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
type Store[T any] struct {
	newEncoder func(io.Writer) Encoder
	newDecoder func(io.Reader) Decoder
}

func New[T any, E Encoder, D Decoder](newEncoder func(io.Writer) E, newDecoder func(io.Reader) D) *Store[T] {
	return &Store[T]{
		newEncoder: func(w io.Writer) Encoder { return newEncoder(w) },
		newDecoder: func(r io.Reader) Decoder { return newDecoder(r) },
	}
}

// Load reads the contents of the file at path and unmarshals it into v.
//
// Load may block if another store is in the process of writing to the file.
func (store *Store[T]) Load(ctx context.Context, path string, v *T) (canary any, err error) {

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	rdf, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer rdf.Close()

	if err := RLock(ctx, rdf); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if err := store.newDecoder(rdf).Decode(v); err != nil {
		return nil, err
	}

	newCanary, err := lstatIno(rdf, "")
	if err != nil {
		return nil, err
	}

	return newCanary, nil
}

// Store marshals v and writes the result into the specified path, overwriting
// its contents. This write is atomic: either all of the data has been written,
// or none of it, in which case the destination remains untouched.
// This prevents all situations where a crashing process leaves the file
// half-written and corrupt.
//
// Store may block if another store is in the process of reading the file.
func (store *Store[T]) Store(ctx context.Context, path string, mode os.FileMode, v *T, canary any) (err error) {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Write the updated contents to an alternate file, then atomically
	// swap it with the original. This avoid corrupting the store should
	// the process terminate mid-write.

	wf, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE, mode&^os.ModeType)
	if err != nil {
		return err
	}
	defer wf.Close()

	if err := Lock(ctx, wf); err != nil {
		return err
	}

	oldCanary, _ := canary.(uint64)
	newCanary, err := lstatIno(nil, path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Compare canaries -- we use inodes as canaries, so an inode of 0 means
	// the file was missing.
	if newCanary != oldCanary {
		// The destination changed while we were waiting for the lock. This
		// means that another concurrent store completed, and we need
		// to retry.
		return ErrRetry
	}

	if ko, err := deleted(wf); ko {
		if err == nil {
			// Another process pulled the rug from under us; we managed to acquire an
			// exclusive lock, but that lock is held on the final file, not the
			// temporary .lock file.
			//
			// In other words, we only acquired the lock after another call to Store
			// finished atomically swapping the result.
			//
			// There's nothing we can do except return ErrRetry.
			err = ErrRetry
		}
		return err
	}

	if err := os.Truncate(wf.Name(), 0); err != nil {
		return err
	}

	if err := store.newEncoder(wf).Encode(v); err != nil {
		return err
	}

	return os.Rename(wf.Name(), path)
}

// LoadAndStoreFunc is the signature of the user callback called by LoadAndStore.
//
// LoadAndStore calls the function with val set to a non-nil pointer to the
// value that was unmarshaled from the content of the specified file.
//
// If the value fails to load (commonly, because the file does not exist, or
// less commonly, because the file fails to unmarshal), the function is still
// called with val set to a pointer to the zero value of T, and err is set to
// the error that occured during loading.
type LoadAndStoreFunc[T any] func(ctx context.Context, val *T, err error) error

func (store *Store[T]) tryLoadAndStore(ctx context.Context, path string, mode os.FileMode, fn LoadAndStoreFunc[T]) error {
	var value T

	canary, err := store.Load(ctx, path, &value)

	if err := fn(ctx, &value, err); err != nil {
		return err
	}

	return store.Store(ctx, path, mode, &value, canary)
}

// LoadAndStore loads the file at path and calls the specified function with the
// result of that load, as if store.Load(ctx, path, &v) was called.
//
// The user function is then free to modify that value. If it returns without
// an error, LoadAndStore attempts to store the value back into the file.
//
// If the underlying file did not change since it first loaded, the store succeeds.
// Otherwise, it is aborted, and the process is retried, reloading the file and
// calling the user function for re-modification.
//
// In effect, LoadAndStore has Compare-and-Swap semantics; the function is preferred
// over Load and Store when the caller needs to update partially the contents of
// the file.
func (store *Store[T]) LoadAndStore(ctx context.Context, path string, mode os.FileMode, fn LoadAndStoreFunc[T]) error {
	err := ErrRetry
	for err == ErrRetry {
		err = store.tryLoadAndStore(ctx, path, mode, fn)
	}
	return err
}
