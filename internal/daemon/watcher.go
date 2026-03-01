package daemon

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// ignoredDirs are directory names that fsnotify should never recurse into.
var ignoredDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"__pycache__":  true,
	".next":        true,
	".nuxt":        true,
	"coverage":     true,
	"vendor":       true,
	".breaklog":    true,
	".cache":       true,
}

// ignoredExts are file extensions to skip.
var ignoredExts = map[string]bool{
	".pyc": true,
	".pyo": true,
	".exe": true,
	".dll": true,
	".so":  true,
}

const maxFileSize = 1 << 20 // 1 MB

// isIgnored returns true for paths that should not trigger session activity.
func isIgnored(path string) bool {
	// Check each path component for ignored directory names.
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if ignoredDirs[part] {
			return true
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ignoredExts[ext] {
		return true
	}

	// Ignore files over 1 MB (likely binary / generated).
	if info, err := os.Stat(path); err == nil && info.Size() > maxFileSize {
		return true
	}

	return false
}

// addWatchRecursive adds path and all non-ignored subdirectories to the watcher.
func addWatchRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if !info.IsDir() {
			return nil
		}
		if ignoredDirs[filepath.Base(path)] {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}
