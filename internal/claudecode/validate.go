package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sensitiveDirs are paths that should never be used as work directories.
var sensitiveDirs = []string{
	"/etc", "/root", "/var", "/usr", "/bin", "/sbin",
	"/System", "/Library", "/private",
}

// ValidateWorkDir checks that a work directory path is safe to use.
func ValidateWorkDir(path string) error {
	if path == "" {
		return fmt.Errorf("work_dir is required")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("work_dir must be an absolute path")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("work_dir must not contain '..'")
	}

	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("work_dir does not exist: %s", path)
		}
		return fmt.Errorf("resolve work_dir: %w", err)
	}

	// Check it's a directory
	info, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("stat work_dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("work_dir is not a directory: %s", path)
	}

	// Reject sensitive directories
	for _, sensitive := range sensitiveDirs {
		if resolved == sensitive || strings.HasPrefix(resolved, sensitive+"/") {
			return fmt.Errorf("work_dir points to sensitive directory: %s", sensitive)
		}
	}

	return nil
}

// ValidateSlug checks that a project slug is valid (lowercase alphanumeric + hyphens).
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug is required")
	}
	if len(slug) > 100 {
		return fmt.Errorf("slug must be 100 characters or less")
	}
	for _, c := range slug {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return fmt.Errorf("slug must contain only lowercase letters, numbers, and hyphens")
		}
	}
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return fmt.Errorf("slug must not start or end with a hyphen")
	}
	return nil
}
