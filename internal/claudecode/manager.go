package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ErrMaxSessionsReached indicates the project has hit its concurrent session limit.
var ErrMaxSessionsReached = fmt.Errorf("max concurrent sessions reached")

// ManagedProcess tracks a running claude CLI child process.
type ManagedProcess struct {
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	sessionID uuid.UUID
	projectID uuid.UUID
	done      chan struct{}
}

// ProcessManager manages Claude Code CLI child processes.
type ProcessManager struct {
	processes map[uuid.UUID]*ManagedProcess
	mu        sync.RWMutex
	store     store.CCStore
	eventCB   EventCallback
}

func NewProcessManager(ccStore store.CCStore, eventCB EventCallback) *ProcessManager {
	return &ProcessManager{
		processes: make(map[uuid.UUID]*ManagedProcess),
		store:     ccStore,
		eventCB:   eventCB,
	}
}

// Start spawns a new claude CLI process. Returns the CC session UUID.
func (m *ProcessManager) Start(ctx context.Context, opts StartOpts, startedBy string) (uuid.UUID, error) {
	// Validate max_sessions limit
	active, err := m.store.ActiveSessionCount(ctx, opts.ProjectID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("check active sessions: %w", err)
	}
	proj, err := m.store.GetProject(ctx, opts.ProjectID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get project: %w", err)
	}
	if active >= proj.MaxSessions {
		return uuid.Nil, ErrMaxSessionsReached
	}

	// Validate work directory
	if err := ValidateWorkDir(opts.WorkDir); err != nil {
		return uuid.Nil, fmt.Errorf("invalid work_dir: %w", err)
	}

	// Create session record
	sess := &store.CCSessionData{
		ProjectID: opts.ProjectID,
		Label:     opts.Prompt,
		Status:    store.CCSessionStatusStarting,
		StartedBy: startedBy,
	}
	if len(opts.Prompt) > 200 {
		sess.Label = opts.Prompt[:200] + "..."
	}
	if err := m.store.CreateSession(ctx, sess); err != nil {
		return uuid.Nil, fmt.Errorf("create session: %w", err)
	}

	// Determine work directory (worktree if needed)
	workDir := opts.WorkDir
	if opts.UseWorktree || active > 0 {
		wtPath, wtErr := CreateWorktree(opts.WorkDir, sess.ID)
		if wtErr != nil {
			slog.Warn("worktree creation failed, using direct workdir", "error", wtErr)
		} else {
			workDir = wtPath
		}
	}

	// Build command args
	args := buildCLIArgs(opts)

	// Prepend scope hint to prompt if set
	prompt := opts.Prompt
	if opts.Scope != "" {
		prompt = fmt.Sprintf("IMPORTANT: Focus only on files matching: %s. Do not modify files outside this scope.\n\n%s", opts.Scope, prompt)
	}

	// Use Background so the child process outlives the HTTP request context
	procCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, "claude", args...)
	cmd.Dir = workDir
	cmd.Env = buildEnv(opts.EnvVars)
	cmd.Stdin = strings.NewReader(prompt)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return uuid.Nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return uuid.Nil, fmt.Errorf("start claude: %w", err)
	}

	// Update session with PID
	pid := cmd.Process.Pid
	_ = m.store.UpdateSession(ctx, sess.ID, map[string]any{
		"status": store.CCSessionStatusRunning,
		"pid":    pid,
	})

	mp := &ManagedProcess{
		cmd:       cmd,
		cancel:    cancel,
		sessionID: sess.ID,
		projectID: opts.ProjectID,
		done:      make(chan struct{}),
	}

	m.mu.Lock()
	m.processes[sess.ID] = mp
	m.mu.Unlock()

	// Reader goroutine: parse stream-json lines
	go func() {
		defer close(mp.done)
		defer func() {
			m.mu.Lock()
			delete(m.processes, sess.ID)
			m.mu.Unlock()
		}()

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer
		var claudeSessionID string

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			event, parseErr := parseStreamLine(line)
			if parseErr != nil {
				slog.Debug("cc: skip unparseable line", "error", parseErr)
				continue
			}

			// Capture claude session_id from init event
			if event.Type == "system" && event.Subtype == "init" && event.SessionID != "" {
				claudeSessionID = event.SessionID
				_ = m.store.UpdateSession(context.Background(), sess.ID, map[string]any{
					"claude_session_id": claudeSessionID,
				})
			}

			// Update tokens/cost from result events
			if event.Type == "result" {
				updates := map[string]any{
					"input_tokens":  event.InputTokens,
					"output_tokens": event.OutputTokens,
					"cost_usd":      event.CostUSD,
				}
				if event.Subtype == "success" {
					updates["status"] = store.CCSessionStatusCompleted
					updates["stopped_at"] = time.Now().UTC()
				} else if event.Subtype == "error" {
					updates["status"] = store.CCSessionStatusFailed
					updates["stopped_at"] = time.Now().UTC()
				}
				_ = m.store.UpdateSession(context.Background(), sess.ID, updates)
			}

			// Forward event to callback
			if m.eventCB != nil {
				m.eventCB(sess.ID, event)
			}
		}

		// Wait for process to exit
		waitErr := cmd.Wait()
		finalStatus := store.CCSessionStatusCompleted
		var errStr *string
		if waitErr != nil {
			finalStatus = store.CCSessionStatusFailed
			s := waitErr.Error()
			if stderr := stderrBuf.String(); stderr != "" {
				s += ": " + stderr
				slog.Warn("cc: process stderr", "session_id", sess.ID, "stderr", stderr)
			}
			errStr = &s
		}

		updates := map[string]any{
			"stopped_at": time.Now().UTC(),
			"pid":        nil,
		}
		// Only override status if not already set by result event
		currentSess, getErr := m.store.GetSession(context.Background(), sess.ID)
		if getErr == nil && currentSess.Status == store.CCSessionStatusRunning {
			updates["status"] = finalStatus
			if errStr != nil {
				updates["error"] = *errStr
			}
		}
		_ = m.store.UpdateSession(context.Background(), sess.ID, updates)
	}()

	slog.Info("cc: process started", "session_id", sess.ID, "project", proj.Name, "pid", pid)
	return sess.ID, nil
}

// Stop sends SIGTERM to a running process, waits up to 10s, then SIGKILL.
func (m *ProcessManager) Stop(ctx context.Context, sessionID uuid.UUID) error {
	m.mu.RLock()
	mp, ok := m.processes[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session %s: no running process", sessionID)
	}

	// Send SIGTERM
	if mp.cmd.Process != nil {
		_ = mp.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait up to 10s, then force kill
	select {
	case <-mp.done:
		// Process exited gracefully
	case <-time.After(10 * time.Second):
		if mp.cmd.Process != nil {
			_ = mp.cmd.Process.Kill()
		}
		<-mp.done
	}

	_ = m.store.UpdateSession(ctx, sessionID, map[string]any{
		"status":     store.CCSessionStatusStopped,
		"stopped_at": time.Now().UTC(),
		"pid":        nil,
	})

	slog.Info("cc: process stopped", "session_id", sessionID)
	return nil
}

// StopAll stops all running processes (used during gateway shutdown).
func (m *ProcessManager) StopAll() {
	m.mu.RLock()
	ids := make([]uuid.UUID, 0, len(m.processes))
	for id := range m.processes {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		_ = m.Stop(context.Background(), id)
	}
}

// SendPrompt sends a follow-up prompt by stopping the current process and restarting with --resume.
func (m *ProcessManager) SendPrompt(ctx context.Context, sessionID uuid.UUID, prompt string) error {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	proj, err := m.store.GetProject(ctx, sess.ProjectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	// Stop current process if running
	if m.IsRunning(sessionID) {
		if err := m.Stop(ctx, sessionID); err != nil {
			slog.Warn("cc: stop before resume failed", "error", err)
		}
	}

	resumeID := ""
	if sess.ClaudeSessionID != nil {
		resumeID = *sess.ClaudeSessionID
	}
	if resumeID == "" {
		return fmt.Errorf("no claude session_id to resume")
	}

	// Parse allowed tools from project
	var allowedTools []string
	if proj.AllowedTools != nil {
		_ = json.Unmarshal(proj.AllowedTools, &allowedTools)
	}

	// Start new process with --resume
	newOpts := StartOpts{
		ProjectID:    sess.ProjectID,
		WorkDir:      proj.WorkDir,
		Prompt:       prompt,
		ResumeID:     resumeID,
		AllowedTools: allowedTools,
	}

	newID, err := m.Start(ctx, newOpts, sess.StartedBy)
	if err != nil {
		return fmt.Errorf("restart with resume: %w", err)
	}

	// Link new session to old claude_session_id
	_ = m.store.UpdateSession(ctx, newID, map[string]any{
		"claude_session_id": resumeID,
	})

	return nil
}

// IsRunning returns true if the session has an active process.
func (m *ProcessManager) IsRunning(sessionID uuid.UUID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.processes[sessionID]
	return ok
}

// ActiveCount returns number of running processes for a project.
func (m *ProcessManager) ActiveCount(projectID uuid.UUID) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, mp := range m.processes {
		if mp.projectID == projectID {
			count++
		}
	}
	return count
}

// buildCLIArgs constructs the claude CLI arguments.
func buildCLIArgs(opts StartOpts) []string {
	args := []string{"-p", "-", "--output-format", "stream-json", "--verbose"}

	if opts.ResumeID != "" {
		args = append(args, "--resume", opts.ResumeID)
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	return args
}

// buildEnv creates environment for the child process.
// Strips CLAUDECODE env var to avoid nested session detection.
func buildEnv(extra map[string]string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			continue
		}
		env = append(env, e)
	}
	for k, v := range extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
