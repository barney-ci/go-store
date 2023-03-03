// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

//go:build !go1.20
// +build !go1.20

package store

import (
	"errors"
)

// Go, before 1.20, doesn't support Unwrap() []error, and we can't implement
// both signatures. Therefore, we emulate the effect of Unwrap() []error
// for likeError by implementing a custom Is.

func (e *likeError) Is(err error) bool {
	switch {
	case errors.Is(err, e.Err):
		return true
	case errors.Is(err, e.Like):
		return true
	}
	return false
}

func (e *likeError) Unwrap() error {
	return e.Err
}
