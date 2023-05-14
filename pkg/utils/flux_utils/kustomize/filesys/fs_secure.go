/*
Copyright 2022 The Flux authors

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

package filesys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

var (
	// tmpConfirmedDirPrefix is the prefix used by filesys.NewTmpConfirmedDir(),
	// the absolute prefix is the value prefixed with os.TempDir().
	tmpConfirmedDirPrefix = "kustomize-"
)

// MakeFsOnDiskSecure returns a secure file system which asserts any paths it
// handles to be inside root. When allowed prefixes are provided, followed
// absolute links are allowed to traverse into these directories.
// The root is not allowed to match an allowed prefix, this to ensure an FS
// instance can not reach into another FS when the allowed prefix configuration
// is static.
func MakeFsOnDiskSecure(root string, allowPrefixes ...string) (filesys.FileSystem, error) {
	unsafeFS := filesys.MakeFsOnDisk()
	cleanedAbs, _, err := unsafeFS.CleanedAbs(root)
	if err != nil {
		return nil, err
	}
	if ok, prefix := hasOneOfPrefixes(cleanedAbs.String(), allowPrefixes); ok {
		return nil, fmt.Errorf("root '%s' cannot be prefixed with '%s'", root, prefix)
	}
	return fsSecure{root: cleanedAbs, unsafeFS: unsafeFS, allowPrefixes: allowPrefixes}, nil
}

// TmpConfirmedDirPrefix returns the absolute prefix path as constructed by
// filesys.NewTmpConfirmedDir().
func TmpConfirmedDirPrefix() (string, error) {
	// Some OS-es seem to symlink TempDir.
	evaluated, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", err
	}
	return filepath.Join(evaluated, tmpConfirmedDirPrefix), nil
}

// MakeFsOnDiskSecureBuild calls MakeFsOnDiskSecure, but configures the
// TmpConfirmedDirPrefix as an allowed prefix.
// NOTE: When e.g. being able to load remote bases is not a concern, opt to use
// MakeFsOnDiskSecure instead, unless running into specific traversal issues.
func MakeFsOnDiskSecureBuild(root string, allowPrefixes ...string) (filesys.FileSystem, error) {
	dirPrefix, err := TmpConfirmedDirPrefix()
	if err != nil {
		return nil, err
	}
	allowPrefixes = append([]string{dirPrefix}, allowPrefixes...)
	return MakeFsOnDiskSecure(root, allowPrefixes...)
}

// fsSecure wraps an unsafe FileSystem implementation, and secures it
// by confirming paths are inside root.
type fsSecure struct {
	root          filesys.ConfirmedDir
	unsafeFS      filesys.FileSystem
	allowPrefixes []string
}

// ConstraintError records an error and the operation and file that
// violated it.
type ConstraintError struct {
	Op   string
	Path string
	Err  error
}

func (e *ConstraintError) Error() string {
	return "fs-security-constraint " + e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *ConstraintError) Unwrap() error { return e.Err }

// Create delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, an error
// of type ConstraintError is returned.
func (fs fsSecure) Create(path string) (filesys.File, error) {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return nil, &ConstraintError{Op: "create", Path: path, Err: err}
	}
	return fs.unsafeFS.Create(path)
}

// Mkdir delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, an error
// of type ConstraintError is returned.
func (fs fsSecure) Mkdir(path string) error {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return &ConstraintError{Op: "mkdir", Path: path, Err: err}
	}
	return fs.unsafeFS.Mkdir(path)
}

// MkdirAll delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, an error
// type ConstraintError is returned.
func (fs fsSecure) MkdirAll(path string) error {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return &ConstraintError{Op: "mkdir", Path: path, Err: err}
	}
	return fs.unsafeFS.MkdirAll(path)
}

// RemoveAll delegates to the embedded unsafe FS after having confirmed the
// path to be inside root. If the provided path violates this constraint, an
// error of type ConstraintError is returned.
func (fs fsSecure) RemoveAll(path string) error {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return &ConstraintError{Op: "remove", Path: path, Err: err}
	}
	return fs.unsafeFS.RemoveAll(path)
}

// Open delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, an error
// of type ConstraintError is returned.
func (fs fsSecure) Open(path string) (filesys.File, error) {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return nil, &ConstraintError{Op: "open", Path: path, Err: err}
	}
	return fs.unsafeFS.Open(path)
}

// IsDir delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, it returns
// false.
func (fs fsSecure) IsDir(path string) bool {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return false
	}
	return fs.unsafeFS.IsDir(path)
}

// ReadDir delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, an error
// of type ConstraintError is returned.
func (fs fsSecure) ReadDir(path string) ([]string, error) {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return nil, &ConstraintError{Op: "open", Path: path, Err: err}
	}
	return fs.unsafeFS.ReadDir(path)
}

// CleanedAbs delegates to the embedded unsafe FS, but confirms the returned
// result to be within root. If the results violates this constraint, an error
// of type ConstraintError is returned.
// In essence, it functions the same as Kustomize's loader.RestrictionRootOnly,
// but on FS levels, and while allowing file paths.
func (fs fsSecure) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	d, f, err := fs.unsafeFS.CleanedAbs(path)
	if err != nil {
		return d, f, err
	}
	if ok, _ := hasOneOfPrefixes(d.String(), fs.allowPrefixes); ok {
		return d, f, err
	}
	if !d.HasPrefix(fs.root) {
		return "", "", &ConstraintError{Op: "abs", Path: path, Err: rootConstraintErr(path, fs.root.String())}
	}
	return d, f, err
}

// Exists delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, it returns
// false.
func (fs fsSecure) Exists(path string) bool {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return false
	}
	return fs.unsafeFS.Exists(path)
}

// Glob delegates to the embedded unsafe FS, but filters the returned paths to
// only include items inside root.
func (fs fsSecure) Glob(pattern string) ([]string, error) {
	paths, err := fs.unsafeFS.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var securePaths []string
	for _, p := range paths {
		if err := isSecurePath(fs.unsafeFS, fs.root, p, fs.allowPrefixes...); err == nil {
			securePaths = append(securePaths, p)
		}
	}
	return securePaths, err
}

// ReadFile delegates to the embedded unsafe FS after having confirmed the path
// to be inside root. If the provided path violates this constraint, an error
// of type ConstraintError is returned.
func (fs fsSecure) ReadFile(path string) ([]byte, error) {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return nil, &ConstraintError{Op: "read", Path: path, Err: err}
	}
	return fs.unsafeFS.ReadFile(path)
}

// WriteFile delegates to the embedded unsafe FS after having confirmed the
// path to be inside root. If the provided path violates this constraint, an
// error of type ConstraintError is returned.
func (fs fsSecure) WriteFile(path string, data []byte) error {
	if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
		return &ConstraintError{Op: "write", Path: path, Err: err}
	}
	return fs.unsafeFS.WriteFile(path, data)
}

// Walk delegates to the embedded unsafe FS, wrapping falkFn in a callback which
// confirms the path to be inside root. If the path violates this constraint,
// an error of type ConstraintError is returned and walkFn is not called.
func (fs fsSecure) Walk(path string, walkFn filepath.WalkFunc) error {
	wrapWalkFn := func(path string, info os.FileInfo, err error) error {
		if err := isSecurePath(fs.unsafeFS, fs.root, path, fs.allowPrefixes...); err != nil {
			return &ConstraintError{Op: "walk", Path: path, Err: err}
		}
		return walkFn(path, info, err)
	}
	return fs.unsafeFS.Walk(path, wrapWalkFn)
}

// isSecurePath confirms the given path is inside root using the provided file
// system. At present, it assumes the file system implementation to be on disk
// and makes use of filepath.EvalSymlinks.
func isSecurePath(fs filesys.FileSystem, root filesys.ConfirmedDir, path string, allowedPrefixes ...string) error {
	absRoot, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs path error on '%s': %v", path, err)
	}
	d := filesys.ConfirmedDir(absRoot)
	if fs.Exists(absRoot) {
		evaluated, err := filepath.EvalSymlinks(absRoot)
		if err != nil {
			return fmt.Errorf("evalsymlink failure on '%s': %w", path, err)
		}
		evaluatedDir := evaluated
		if !fs.IsDir(evaluatedDir) {
			evaluatedDir = filepath.Dir(evaluatedDir)
		}
		d = filesys.ConfirmedDir(evaluatedDir)
	}
	if ok, _ := hasOneOfPrefixes(d.String(), allowedPrefixes); ok {
		return nil
	}
	if !d.HasPrefix(root) {
		return rootConstraintErr(path, root.String())
	}
	return nil
}

func rootConstraintErr(path, root string) error {
	return fmt.Errorf("path '%s' is not in or below '%s'", path, root)
}

func hasOneOfPrefixes(s string, prefixes []string) (bool, string) {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true, p
		}
	}
	return false, ""
}
