package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type IgnoreMatcher struct {
	patterns []string
}

// CompileGlobToRegex converts a standard git-style glob pattern to a Go regexp
func CompileGlobToRegex(pattern string) (*regexp.Regexp, error) {
	// Standardize slashes to forward slash
	pattern = filepath.ToSlash(pattern)
	pattern = strings.TrimSpace(pattern)

	// Build the regex string
	var sb strings.Builder
	sb.WriteString("^")

	// If the pattern doesn't contain a slash, it can match at any directory depth.
	// For example, "tmp" matches "tmp", "dir/tmp", "dir1/dir2/tmp".
	// To achieve this, if there's no slash (except a trailing one), prepend "(.*/)?".
	hasSlash := strings.Contains(strings.TrimSuffix(pattern, "/"), "/")
	if !hasSlash && !strings.HasPrefix(pattern, "/") {
		sb.WriteString("(?:.*/)?")
	}

	// Remove leading slash for matching since we'll clean paths to be relative
	pattern = strings.TrimPrefix(pattern, "/")

	i := 0
	for i < len(pattern) {
		if i+3 <= len(pattern) && pattern[i:i+3] == "**/" {
			sb.WriteString("(?:.*/)?")
			i += 3
		} else if i+2 <= len(pattern) && pattern[i:i+2] == "**" {
			sb.WriteString(".*")
			i += 2
		} else if pattern[i] == '*' {
			sb.WriteString("[^/]*")
			i++
		} else if pattern[i] == '?' {
			sb.WriteString("[^/]")
			i++
		} else if strings.ContainsRune(".+()|{}^$[]\\", rune(pattern[i])) {
			sb.WriteByte('\\')
			sb.WriteByte(pattern[i])
			i++
		} else {
			sb.WriteByte(pattern[i])
			i++
		}
	}

	// If the pattern ended with a slash (or we want directory matching), we match the directory or any contents
	if strings.HasSuffix(pattern, "/") {
		sb.WriteString(".*")
	} else {
		// Or if it matches a directory exactly, it also matches everything inside it
		sb.WriteString("(?:/.*)?$")
	}

	return regexp.Compile(sb.String())
}

// NewIgnoreMatcher creates a matcher from a list of patterns, adding default ignores
func NewIgnoreMatcher(baseDir string) *IgnoreMatcher {
	m := &IgnoreMatcher{
		patterns: []string{
			".git/",
			".tsync-config/",
		},
	}

	// Read .tsyncignore file if it exists
	ignorePath := filepath.Join(baseDir, ".tsyncignore")
	file, err := os.Open(ignorePath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Ignore empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			m.patterns = append(m.patterns, line)
		}
	}

	return m
}

// Matches returns true if the relative file path should be ignored
func (im *IgnoreMatcher) Matches(relPath string) bool {
	// Standardize to forward slashes for matching consistency
	relPath = filepath.ToSlash(relPath)
	relPath = strings.TrimPrefix(relPath, "./")

	for _, p := range im.patterns {
		re, err := CompileGlobToRegex(p)
		if err != nil {
			continue
		}
		if re.MatchString(relPath) {
			return true
		}
	}

	return false
}
