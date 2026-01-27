package hubclient

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestBuilder builds a template manifest from local files.
type ManifestBuilder struct {
	// BasePath is the root directory of the template.
	BasePath string
	// IgnorePatterns are glob patterns to ignore.
	IgnorePatterns []string
}

// NewManifestBuilder creates a new manifest builder.
func NewManifestBuilder(basePath string) *ManifestBuilder {
	return &ManifestBuilder{
		BasePath: basePath,
		IgnorePatterns: []string{
			".git",
			".git/**",
			".DS_Store",
			"**/.DS_Store",
		},
	}
}

// Build walks the template directory and builds a manifest.
func (b *ManifestBuilder) Build() (*TemplateManifest, error) {
	var files []TemplateFile

	err := filepath.WalkDir(b.BasePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(b.BasePath, path)
		if err != nil {
			return err
		}

		// Use forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		// Skip root
		if relPath == "." {
			return nil
		}

		// Check ignore patterns
		if b.shouldIgnore(relPath, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Compute hash and get file info
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", relPath, err)
		}

		hash, err := b.hashFile(path)
		if err != nil {
			return fmt.Errorf("failed to hash file %s: %w", relPath, err)
		}

		// Get file mode
		mode := fmt.Sprintf("%04o", info.Mode().Perm())

		files = append(files, TemplateFile{
			Path: relPath,
			Size: info.Size(),
			Hash: hash,
			Mode: mode,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files by path for deterministic manifest
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return &TemplateManifest{
		Version: "1.0",
		Files:   files,
	}, nil
}

// shouldIgnore checks if a path should be ignored.
func (b *ManifestBuilder) shouldIgnore(relPath string, isDir bool) bool {
	for _, pattern := range b.IgnorePatterns {
		// Handle ** patterns
		if strings.Contains(pattern, "**") {
			// Convert ** pattern to check
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(relPath, prefix+"/") || relPath == prefix {
				return true
			}
			// Check if pattern matches directory contents
			suffix := strings.TrimPrefix(pattern, "**/")
			if suffix != pattern && strings.HasSuffix(relPath, suffix) {
				return true
			}
			continue
		}

		// Simple match
		if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
	}
	return false
}

// hashFile computes the SHA-256 hash of a file.
func (b *ManifestBuilder) hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeContentHash computes the overall content hash from file hashes.
func ComputeContentHash(files []TemplateFile) string {
	// Sort files by path for deterministic ordering
	sorted := make([]TemplateFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	// Concatenate hashes and compute final hash
	hasher := sha256.New()
	for _, file := range sorted {
		hasher.Write([]byte(file.Hash))
	}

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

// FileInfo contains information about a local file for upload.
type FileInfo struct {
	Path     string // Relative path
	FullPath string // Absolute path
	Size     int64
	Hash     string
	Mode     string
}

// CollectFiles collects file information from a directory for upload.
func CollectFiles(basePath string, ignorePatterns []string) ([]FileInfo, error) {
	builder := NewManifestBuilder(basePath)
	if ignorePatterns != nil {
		builder.IgnorePatterns = append(builder.IgnorePatterns, ignorePatterns...)
	}

	var files []FileInfo

	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if relPath == "." {
			return nil
		}

		if builder.shouldIgnore(relPath, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		hash, err := builder.hashFile(path)
		if err != nil {
			return err
		}

		files = append(files, FileInfo{
			Path:     relPath,
			FullPath: path,
			Size:     info.Size(),
			Hash:     hash,
			Mode:     fmt.Sprintf("%04o", info.Mode().Perm()),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}
