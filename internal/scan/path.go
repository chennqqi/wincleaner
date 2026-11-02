package scan

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatSize renders a byte count for the cleanup list.
func FormatSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div := int64(unit)
	exp := 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// WithinRoots reports whether path is strictly inside one of roots.
// The root directory itself is not a deletion target.
func WithinRoots(path string, roots []string) bool {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return false
	}
	for _, root := range roots {
		root = filepath.Clean(root)
		if root == "" || root == "." {
			continue
		}
		if hasChildPrefix(path, root) {
			return true
		}
	}
	return false
}

// hasChildPrefix reports whether path is a case-insensitive child of root.
func hasChildPrefix(path, root string) bool {
	path = strings.TrimRight(path, `\/`)
	root = strings.TrimRight(root, `\/`)
	if len(path) <= len(root) {
		return false
	}
	if !strings.EqualFold(path[:len(root)], root) {
		return false
	}
	sep := path[len(root)]
	return sep == '\\' || sep == '/'
}

// normalizeRoots drops blanks and case-insensitive duplicates.
func normalizeRoots(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, root := range in {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		key := strings.ToLower(root)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, root)
	}
	return out
}
