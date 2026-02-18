// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist.
func CopyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

// MakeWritableRecursive recursively makes all files and directories in the path writable by the user.
func MakeWritableRecursive(path string) error {
	var totalFiles, chmodCount int
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		totalFiles++
		if info.Mode().Perm()&0200 == 0 {
			chmodCount++
			return os.Chmod(path, info.Mode().Perm()|0200)
		}
		return nil
	})
	Debugf("MakeWritableRecursive: walked %d files, chmod'd %d", totalFiles, chmodCount)
	return err
}

// RemoveAllAsync removes a directory tree without blocking the caller.
// It renames the directory to a unique tombstone name (an instant metadata
// operation), then spawns a background "rm -rf" process to handle the actual
// deletion. This avoids blocking on slow-to-delete files such as symlinks
// pointing to container-internal paths (e.g. /home/scion/...) which can
// trigger macOS autofs timeouts during unlink.
func RemoveAllAsync(path string) error {
	tombstone := fmt.Sprintf("%s.deleting-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, tombstone); err != nil {
		Debugf("RemoveAllAsync: rename failed, falling back to sync removal: %v", err)
		return os.RemoveAll(path)
	}

	Debugf("RemoveAllAsync: renamed %s -> %s", filepath.Base(path), filepath.Base(tombstone))

	cmd := exec.Command("rm", "-rf", tombstone)
	if err := cmd.Start(); err != nil {
		Debugf("RemoveAllAsync: background rm failed to start, falling back to sync: %v", err)
		return os.RemoveAll(tombstone)
	}

	// Reap the child process to prevent zombies if we outlive it.
	go cmd.Wait()

	Debugf("RemoveAllAsync: background rm started (pid %d)", cmd.Process.Pid)
	return nil
}

// CleanupPendingDeletions removes leftover tombstone directories in dir
// from previous async deletions that may not have completed.
func CleanupPendingDeletions(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.Contains(e.Name(), ".deleting-") {
			continue
		}
		tombstone := filepath.Join(dir, e.Name())
		Debugf("CleanupPendingDeletions: removing leftover %s", e.Name())
		cmd := exec.Command("rm", "-rf", tombstone)
		if err := cmd.Start(); err != nil {
			go os.RemoveAll(tombstone)
		} else {
			go cmd.Wait()
		}
	}
}
