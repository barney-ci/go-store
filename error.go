// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store

type likeError struct {
	Err, Like error
}

func (e *likeError) Error() string {
	return e.Err.Error()
}
