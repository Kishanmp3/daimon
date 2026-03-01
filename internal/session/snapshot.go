package session

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// defaultExcludes are directory/file patterns always ignored when snapshotting.
var defaultExcludes = []string{
	"node_modules",
	".git",
	"dist",
	"build",
	"__pycache__",
	".DS_Store",
	"*.pyc",
	"*.pyo",
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
	"vendor",
	".breaklog",
	".next",
	".nuxt",
	"coverage",
	".cache",
}

// InitShadow ensures the shadow directory exists for a project.
// Safe to call on an already-initialised shadow.
func InitShadow(projectPath, shadowPath string) error {
	return os.MkdirAll(shadowPath, 0o755)
}

// TakeSnapshot copies the current project files into a timestamped directory
// inside shadowPath and returns that directory path as the snapshot identifier.
func TakeSnapshot(projectPath, shadowPath string) (string, error) {
	snapDir := filepath.Join(shadowPath, fmt.Sprintf("snap_%d", time.Now().UnixNano()))
	if err := copyTree(projectPath, snapDir); err != nil {
		return "", err
	}
	return snapDir, nil
}

// GetHeadHash takes a fresh snapshot and returns its directory path as the
// baseline identifier stored in the DB at session start.
func GetHeadHash(shadowPath, projectPath string) (string, error) {
	return TakeSnapshot(projectPath, shadowPath)
}

// GenerateDiff compares the current project files against the snapshot at
// snapshotDir (the path returned by TakeSnapshot / GetHeadHash).
// Returns a unified-style diff, a list of changed file paths, and any error.
func GenerateDiff(projectPath, shadowPath, snapshotDir string) (string, []string, error) {
	snapExists := snapshotDir != ""
	if snapExists {
		if _, err := os.Stat(snapshotDir); err != nil {
			snapExists = false
		}
	}

	snapFiles := map[string][]string{}
	if snapExists {
		if err := walkTextFiles(snapshotDir, func(rel string, lines []string) {
			snapFiles[rel] = lines
		}); err != nil {
			return "", nil, fmt.Errorf("read snapshot: %w", err)
		}
	}

	currentFiles := map[string][]string{}
	if err := walkTextFiles(projectPath, func(rel string, lines []string) {
		currentFiles[rel] = lines
	}); err != nil {
		return "", nil, fmt.Errorf("read current project: %w", err)
	}

	// Build sorted union of all relative paths for deterministic output.
	pathSet := map[string]struct{}{}
	for p := range snapFiles {
		pathSet[p] = struct{}{}
	}
	for p := range currentFiles {
		pathSet[p] = struct{}{}
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var sb strings.Builder
	var changedFiles []string

	for _, rel := range paths {
		patch := unifiedDiff(rel, snapFiles[rel], currentFiles[rel])
		if patch == "" {
			continue
		}
		changedFiles = append(changedFiles, rel)
		sb.WriteString(patch)
	}

	diffText := sb.String()
	const maxDiffBytes = 200_000
	if len(diffText) > maxDiffBytes {
		diffText = diffText[:maxDiffBytes] + "\n... (diff truncated)"
	}
	return diffText, changedFiles, nil
}

// ── File walking & copying ────────────────────────────────────────────────────

// isExcluded reports whether a single path component matches any exclude pattern.
func isExcluded(name string) bool {
	for _, pat := range defaultExcludes {
		if matched, _ := filepath.Match(pat, name); matched {
			return true
		}
	}
	return false
}

// copyTree recursively copies non-excluded files from src into dst.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if isExcluded(part) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if info.Size() > 1<<20 { // skip files > 1 MB
			return nil
		}
		return copyFile(path, filepath.Join(dst, rel))
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// walkTextFiles calls fn for each non-excluded text file under root, passing
// its slash-separated relative path and its lines. Binary/unreadable files
// are silently skipped.
func walkTextFiles(root string, fn func(rel string, lines []string)) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if isExcluded(part) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			return nil
		}
		if info.Size() > 1<<20 {
			return nil
		}
		lines, err := readLines(path)
		if err != nil {
			return nil // binary or unreadable — skip
		}
		fn(filepath.ToSlash(rel), lines)
		return nil
	})
}

// readLines reads a text file into a slice of lines (without trailing newlines).
// Returns an error if the file appears to be binary (contains NUL bytes).
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<16)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.ContainsRune(line, 0) {
			return nil, fmt.Errorf("binary file")
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

// ── Diff engine ───────────────────────────────────────────────────────────────

const contextLines = 3

// unifiedDiff produces a unified-diff string for a single file.
// aLines = snapshot (old), bLines = current (new). Returns "" if identical.
func unifiedDiff(relPath string, aLines, bLines []string) string {
	if slicesEqual(aLines, bLines) {
		return ""
	}

	var sb strings.Builder

	switch {
	case len(aLines) == 0:
		// New file.
		sb.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n",
			relPath, len(bLines)))
		for _, l := range bLines {
			sb.WriteString("+" + l + "\n")
		}
	case len(bLines) == 0:
		// Deleted file.
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ /dev/null\n@@ -1,%d +0,0 @@\n",
			relPath, len(aLines)))
		for _, l := range aLines {
			sb.WriteString("-" + l + "\n")
		}
	default:
		ops := diffOps(aLines, bLines)
		hunks := buildHunks(ops, aLines, bLines)
		if len(hunks) == 0 {
			return ""
		}
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", relPath, relPath))
		for _, h := range hunks {
			sb.WriteString(h)
		}
	}
	return sb.String()
}

// diffOp represents a single diff operation.
type diffOp struct {
	kind byte // '=' equal, '+' insert, '-' delete
	aIdx int
	bIdx int
}

// diffOps computes an edit script between a and b via LCS.
// Files longer than 500 lines are diffed as a wholesale replacement to avoid
// O(n²) memory usage.
func diffOps(a, b []string) []diffOp {
	const maxLines = 500
	if len(a) > maxLines || len(b) > maxLines {
		ops := make([]diffOp, 0, len(a)+len(b))
		for i := range a {
			ops = append(ops, diffOp{'-', i, -1})
		}
		for j := range b {
			ops = append(ops, diffOp{'+', -1, j})
		}
		return ops
	}

	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce the op sequence.
	var ops []diffOp
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			ops = append(ops, diffOp{'=', i - 1, j - 1})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			ops = append(ops, diffOp{'+', -1, j - 1})
			j--
		default:
			ops = append(ops, diffOp{'-', i - 1, -1})
			i--
		}
	}
	// Reverse: backtracking produces ops in reverse order.
	for lo, hi := 0, len(ops)-1; lo < hi; lo, hi = lo+1, hi-1 {
		ops[lo], ops[hi] = ops[hi], ops[lo]
	}
	return ops
}

// buildHunks converts a flat op sequence into unified-diff hunk strings,
// grouping nearby changes with contextLines lines of surrounding context.
func buildHunks(ops []diffOp, a, b []string) []string {
	type span struct{ start, end int } // indices into ops

	// Find spans of changed ops.
	var changed []span
	i := 0
	for i < len(ops) {
		if ops[i].kind != '=' {
			start := i
			for i < len(ops) && ops[i].kind != '=' {
				i++
			}
			changed = append(changed, span{start, i})
		} else {
			i++
		}
	}

	// Expand each span by contextLines and merge overlapping.
	var merged []span
	for _, s := range changed {
		lo := s.start - contextLines
		if lo < 0 {
			lo = 0
		}
		hi := s.end + contextLines
		if hi > len(ops) {
			hi = len(ops)
		}
		if len(merged) > 0 && lo <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = hi
		} else {
			merged = append(merged, span{lo, hi})
		}
	}

	var hunks []string
	for _, s := range merged {
		slice := ops[s.start:s.end]

		aStart, bStart := -1, -1
		aCount, bCount := 0, 0
		for _, o := range slice {
			if o.kind == '=' || o.kind == '-' {
				if aStart < 0 {
					aStart = o.aIdx
				}
				aCount++
			}
			if o.kind == '=' || o.kind == '+' {
				if bStart < 0 {
					bStart = o.bIdx
				}
				bCount++
			}
		}
		if aStart < 0 {
			aStart = 0
		}
		if bStart < 0 {
			bStart = 0
		}

		var hunk strings.Builder
		hunk.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			aStart+1, aCount, bStart+1, bCount))
		for _, o := range slice {
			switch o.kind {
			case '=':
				hunk.WriteString(" " + a[o.aIdx] + "\n")
			case '-':
				hunk.WriteString("-" + a[o.aIdx] + "\n")
			case '+':
				hunk.WriteString("+" + b[o.bIdx] + "\n")
			}
		}
		hunks = append(hunks, hunk.String())
	}
	return hunks
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
