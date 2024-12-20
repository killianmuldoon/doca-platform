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

//go:generate mockgen -copyright_file ../../../../../hack/boilerplate.go.txt -destination mock/PkgWrapper.go -source mountlib.go
package mountlib

import (
	mountLib "k8s.io/mount-utils"
	kexec "k8s.io/utils/exec"
)

const (
	// DefaultMounter contains path to the mounter tool which should be used by default
	DefaultMounter = "/bin/mount"
	// DefaultProcMountInfoPath is the location of the mountinfo file
	DefaultProcMountInfoPath = "/proc/self/mountinfo"
)

type MountInfo = mountLib.MountInfo

// PkgWrapper is a wrapper for os package from stdlib
type PkgWrapper interface {
	// Mount mounts source to target as fstype with given options.
	// options MUST not contain sensitive material (like passwords).
	Mount(source string, target string, fstype string, options []string) error
	// Unmount unmounts given target.
	Unmount(target string) error
	// ParseMountInfo parses /proc/xxx/mountinfo.
	ParseMountInfo(filename string) ([]MountInfo, error)
	// PathExists returns true if the specified path exists.
	PathExists(path string) (bool, error)
	// IsCorruptedMnt return true if err is about corrupted mount point
	IsCorruptedMnt(err error) bool
	// FormatAndMount formats the given disk, if needed, and mounts it.
	// That is if the disk is not formatted and it is not being mounted as
	// read-only it will format it first then mount it. Otherwise, if the
	// disk is already formatted or it is being mounted as read-only, it
	// will be mounted without formatting.
	FormatAndMount(source string, target string, fstype string, options []string) error
}

// New returns a new instance of the wrapper for k8s.io/mount-utils package
func New(mounter string) PkgWrapper {
	m := mountLib.New(mounter)
	return &mountUtilsWrapper{
		mounter: m,
		safeFormatAndMount: &mountLib.SafeFormatAndMount{
			Interface: m,
			Exec:      kexec.New(),
		},
	}
}

type mountUtilsWrapper struct {
	mounter            mountLib.Interface
	safeFormatAndMount *mountLib.SafeFormatAndMount
}

// Mount is a wrapper for k8s.io/mount-utils Mount
func (w *mountUtilsWrapper) Mount(source string, target string, fstype string, options []string) error {
	return w.mounter.Mount(source, target, fstype, options)
}

// Unmount is a wrapper for k8s.io/mount-utils Unmount
func (w *mountUtilsWrapper) Unmount(target string) error {
	return w.mounter.Unmount(target)
}

// ParseMountInfo is a wrapper for k8s.io/mount-utils ParseMountInfo
func (w *mountUtilsWrapper) ParseMountInfo(filename string) ([]MountInfo, error) {
	return mountLib.ParseMountInfo(filename)
}

// PathExists is a wrapper for k8s.io/mount-utils PathExists
func (w *mountUtilsWrapper) PathExists(path string) (bool, error) {
	return mountLib.PathExists(path)
}

// IsCorruptedMnt is a wrapper for k8s.io/mount-utils IsCorruptedMnt
func (w *mountUtilsWrapper) IsCorruptedMnt(err error) bool {
	return mountLib.IsCorruptedMnt(err)
}

// FormatAndMount is a wrapper for k8s.io/mount-utils FormatAndMount
func (w *mountUtilsWrapper) FormatAndMount(source string, target string, fstype string, options []string) error {
	return w.safeFormatAndMount.FormatAndMount(source, target, fstype, options)
}
