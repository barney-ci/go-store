// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

// The store package provides filesystem locking and concurrency primitives
// to streamline common use-cases when writing programs that access shared
// state through files or directories.
//
// It provides a pure-go, interruptible file lock implementation with the Lock
// type.
package store
