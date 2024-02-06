package fs

import "path/filepath"

// NormalizePaths makes all paths use forward slashes
func NormalizePaths(paths []string) []string {
	normalizedPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		normalizedPaths = append(normalizedPaths, filepath.ToSlash(path))
	}
	return normalizedPaths
}
