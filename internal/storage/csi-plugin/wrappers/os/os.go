/*
Copyright 2024 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:generate mockgen -copyright_file ../../../../../hack/boilerplate.go.txt -destination mock/PkgWrapper.go -source os.go

package os

import "os"

// PkgWrapper is a wrapper for os package from stdlib
type PkgWrapper interface {
	// Stat returns a FileInfo describing the named file.
	// If there is an error, it will be of type *PathError.
	Stat(name string) (os.FileInfo, error)
	// RemoveAll removes path and any children it contains.
	// It removes everything it can but returns the first error
	// it encounters. If the path does not exist, RemoveAll
	// returns nil (no error).
	// If there is an error, it will be of type *PathError.
	RemoveAll(path string) error
	// ReadDir reads the named directory,
	// returning all its directory entries sorted by filename.
	// If an error occurs reading the directory,
	// ReadDir returns the entries it was able to read before the error,
	// along with the error.
	ReadDir(name string) ([]os.DirEntry, error)
	// Readlink returns the destination of the named symbolic link.
	// If there is an error, it will be of type *PathError.
	Readlink(name string) (string, error)
	// ReadFile reads the named file and returns the contents.
	// A successful call returns err == nil, not err == EOF.
	// Because ReadFile reads the whole file, it does not treat an EOF from Read
	// as an error to be reported.
	ReadFile(name string) ([]byte, error)
	// WriteFile writes data to the named file, creating it if necessary.
	// If the file does not exist, WriteFile creates it with permissions perm (before umask);
	// otherwise WriteFile truncates it before writing, without changing permissions.
	WriteFile(name string, data []byte, perm os.FileMode) error
	// MkdirAll creates a directory named path,
	// along with any necessary parents, and returns nil,
	// or else returns an error.
	// The permission bits perm (before umask) are used for all
	// directories that MkdirAll creates.
	// If path is already a directory, MkdirAll does nothing
	// and returns nil.
	MkdirAll(path string, perm os.FileMode) error
	// OpenFile is the generalized open call; most users will use Open
	// or Create instead. It opens the named file with specified flag
	// (O_RDONLY etc.). If the file does not exist, and the O_CREATE flag
	// is passed, it is created with mode perm (before umask). If successful,
	// methods on the returned File can be used for I/O.
	// If there is an error, it will be of type *PathError.
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
}

// New returns a new instance of os package wrapper
func New() PkgWrapper {
	return &osWrapper{}
}

type osWrapper struct{}

// Stat is a wrapper for os.Stat
func (o *osWrapper) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// RemoveAll is a wrapper for os.RemoveAll
func (o *osWrapper) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// ReadDir is a wrapper for os.ReadDir
func (o *osWrapper) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

// Readlink is a wrapper for os.Readlink
func (o *osWrapper) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

// ReadFile is a wrapper for os.ReadFile
func (o *osWrapper) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile is a wrapper for os.WriteFile
func (o *osWrapper) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// MkdirAll is a wrapper for os.MkdirAll
func (o *osWrapper) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// OpenFile is a wrapper for os.OpenFile
func (o *osWrapper) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
