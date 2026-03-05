package claudecode

import (
	"os"
	"testing"
)

func TestValidateWorkDir_Valid(t *testing.T) {
	// Use home dir which is safe and always exists
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	if err := ValidateWorkDir(home); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateWorkDir_Empty(t *testing.T) {
	if err := ValidateWorkDir(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestValidateWorkDir_Relative(t *testing.T) {
	if err := ValidateWorkDir("relative/path"); err == nil {
		t.Error("expected error for relative path")
	}
}

func TestValidateWorkDir_DotDot(t *testing.T) {
	if err := ValidateWorkDir("/tmp/../etc"); err == nil {
		t.Error("expected error for path with ..")
	}
}

func TestValidateWorkDir_NonExistent(t *testing.T) {
	if err := ValidateWorkDir("/nonexistent/path/abc123"); err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestValidateWorkDir_SensitiveDir(t *testing.T) {
	if err := ValidateWorkDir("/etc"); err == nil {
		t.Error("expected error for /etc")
	}
}

func TestValidateWorkDir_SensitiveSubdir(t *testing.T) {
	if err := ValidateWorkDir("/etc/nginx"); err == nil {
		t.Error("expected error for /etc subdir")
	}
}

func TestValidateSlug_Valid(t *testing.T) {
	valid := []string{"my-project", "project1", "a", "test-123-app"}
	for _, s := range valid {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("expected valid slug %q, got error: %v", s, err)
		}
	}
}

func TestValidateSlug_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"My-Project",    // uppercase
		"my project",    // space
		"-leading",      // leading hyphen
		"trailing-",     // trailing hyphen
		"special@chars", // special chars
	}
	for _, s := range invalid {
		if err := ValidateSlug(s); err == nil {
			t.Errorf("expected error for slug %q", s)
		}
	}
}
