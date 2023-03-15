// Copyright 2023 Arista Networks, Inc. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be found
// in the LICENSE file.
//

package store

import (
	"bytes"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInfoEx struct {
	Flags         uint32
	RootDirectory windows.Handle
	FileName      []uint16
}

func (info *fileRenameInfoEx) Bytes() []byte {
	var sys struct {
		Flags          uint32
		RootDirectory  windows.Handle
		FileNameLength uint32
		FileName       [1]uint16
	}
	sys.Flags = info.Flags
	sys.RootDirectory = info.RootDirectory
	sys.FileNameLength = uint32(2 * (len(info.FileName) - 1))

	var data bytes.Buffer
	data.Write(unsafe.Slice((*byte)(unsafe.Pointer(&sys)), unsafe.Offsetof(sys.FileName)))
	data.Write(unsafe.Slice((*byte)(unsafe.Pointer(&info.FileName[0])), len(info.FileName)*2))
	return data.Bytes()
}

func rename(f OSFile, to string) error {

	// os.Rename does not work, because it doesn't replace the destination
	// atomically, nor does it replace it when the destination is already
	// opened by another process, defeating the whole purpose of rename.

	u16path, err := windows.UTF16FromString(to)
	if err != nil {
		return &os.PathError{Op: "UTF16FromString", Path: to, Err: err}
	}

	info := fileRenameInfoEx{
		Flags:    windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS,
		FileName: u16path,
	}
	bytes := info.Bytes()

	err = windows.SetFileInformationByHandle(windows.Handle(f.Fd()), windows.FileRenameInfoEx, (*byte)(unsafe.Pointer(&bytes[0])), uint32(len(bytes)))
	if err != nil {
		return &os.PathError{Op: fmt.Sprintf("rename %s", f.Name()), Path: to, Err: err}
	}
	return nil
}

func openShared(path string, flag int, _ os.FileMode) (*os.File, error) {

	// os.OpenFile is insufficient because Go opens file with FILE_SHARE_READ|FILE_SHARE_WRITE,
	// but not FILE_SHARE_DELETE. This means it's impossible to atomically replace
	// the destination in Load+Store operations.

	u16path, err := windows.UTF16FromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "UTF16FromString", Path: path, Err: err}
	}

	var (
		mode       uint32
		createmode uint32
	)
	switch flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDWR:
		mode = windows.GENERIC_READ | windows.GENERIC_WRITE
	case os.O_RDONLY:
		mode = windows.GENERIC_READ
	case os.O_WRONLY:
		mode = windows.GENERIC_WRITE
	}
	mode |= windows.DELETE
	switch {
	case flag&(os.O_CREATE|os.O_EXCL) == (os.O_CREATE | os.O_EXCL):
		createmode = windows.CREATE_NEW
	case flag&(os.O_CREATE|os.O_TRUNC) == (os.O_CREATE | os.O_TRUNC):
		createmode = windows.CREATE_ALWAYS
	case flag&os.O_CREATE == os.O_CREATE:
		createmode = windows.OPEN_ALWAYS
	case flag&os.O_TRUNC == os.O_TRUNC:
		createmode = windows.TRUNCATE_EXISTING
	default:
		createmode = windows.OPEN_EXISTING
	}

	handle, err := windows.CreateFile(&u16path[0],
		mode,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		createmode,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.Handle(0),
	)
	if err != nil {
		return nil, &os.PathError{Op: "CreateFile", Path: path, Err: err}
	}

	return os.NewFile(uintptr(handle), path), nil
}

func lstatIno(f *os.File, path string) (uint64, error) {
	var info windows.ByHandleFileInformation
	if path == "" {
		if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
			return 0, &os.PathError{Op: "GetFileInformationByHandle", Path: "handle:" + f.Name(), Err: err}
		}
	} else {
		u16path, err := windows.UTF16FromString(path)
		if err != nil {
			return 0, &os.PathError{Op: "UTF16FromString", Path: path, Err: err}
		}

		handle, err := windows.CreateFile(&u16path[0],
			windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
			windows.Handle(0),
		)
		if err != nil {
			return 0, &os.PathError{Op: "CreateFile", Path: path, Err: err}
		}
		defer windows.Close(handle)

		if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
			return 0, &os.PathError{Op: "GetFileInformationByHandle", Path: path, Err: err}
		}
	}
	return uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow), nil
}
