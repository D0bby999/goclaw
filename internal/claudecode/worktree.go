package claudecode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const worktreeDir = ".claude-worktrees"

// CreateWorktree creates an isolated git worktree for a CC session.
// Branch name: cc/<session-uuid-short>
// Path: <projectDir>/.claude-worktrees/<session-uuid-short>
func CreateWorktree(projectDir string, sessionID uuid.UUID) (string, error) {
	short := sessionID.String()[:8]
	branch := "cc/" + short
	wtPath := filepath.Join(projectDir, worktreeDir, short)

	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", fmt.Errorf("create worktree parent: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return wtPath, nil
}

// CleanupWorktree removes the worktree and deletes the branch.
func CleanupWorktree(projectDir string, sessionID uuid.UUID) error {
	short := sessionID.String()[:8]
	branch := "cc/" + short
	wtPath := filepath.Join(projectDir, worktreeDir, short)

	// Remove worktree
	cmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
	cmd.Dir = projectDir
	_ = cmd.Run()

	// Delete branch
	cmd = exec.Command("git", "branch", "-D", branch)
	cmd.Dir = projectDir
	_ = cmd.Run()

	return nil
}
