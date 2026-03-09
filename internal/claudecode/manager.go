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

	"github.com/nextlevelbuilder/goclaw/internal/providers"
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
	startedAt time.Time
}

// ProcessManager manages Claude Code CLI child processes.
type ProcessManager struct {
	processes    map[uuid.UUID]*ManagedProcess
	mu           sync.RWMutex
	store        store.ProjectStore
	eventCB      EventCallback
	gatewayAddr  string
	gatewayToken string
}

func NewProcessManager(ccStore store.ProjectStore, eventCB EventCallback) *ProcessManager {
	return &ProcessManager{
		processes: make(map[uuid.UUID]*ManagedProcess),
		store:     ccStore,
		eventCB:   eventCB,
	}
}

// SetGatewayConfig sets the gateway address and token for MCP bridge integration.
// When set, spawned CLI processes get --mcp-config pointing to GoClaw's MCP bridge,
// --settings with security hooks, and --disallowedTools to disable CLI builtins.
func (m *ProcessManager) SetGatewayConfig(addr, token string) {
	m.gatewayAddr = addr
	m.gatewayToken = token
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
	sess := &store.ProjectSessionData{
		ProjectID: opts.ProjectID,
		Label:     opts.Prompt,
		Status:    store.ProjectSessionStatusStarting,
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

	if err := m.spawnAndWatch(ctx, sess.ID, opts.ProjectID, opts, workDir); err != nil {
		return uuid.Nil, err
	}

	// Start watchdog if project has max_duration set
	if proj.MaxDuration > 0 {
		go m.watchTimeout(sess.ID, time.Duration(proj.MaxDuration)*time.Second)
	}

	slog.Info("cc: process started", "session_id", sess.ID, "project", proj.Name)
	return sess.ID, nil
}

// spawnAndWatch starts a claude CLI process and watches its stdout for stream events.
// Shared by Start (new session) and SendPrompt (resume in same session).
func (m *ProcessManager) spawnAndWatch(ctx context.Context, sessionID, projectID uuid.UUID, opts StartOpts, workDir string) error {
	// Prepend scope hint to prompt if set
	prompt := opts.Prompt
	if opts.Scope != "" {
		prompt = fmt.Sprintf("IMPORTANT: Focus only on files matching: %s. Do not modify files outside this scope.\n\n%s", opts.Scope, prompt)
	}

	// When MCP bridge is active, CLI builtins are disabled via --disallowedTools,
	// so --allowedTools (which refers to CLI builtins) must not be emitted.
	if m.gatewayAddr != "" {
		opts.AllowedTools = nil
	}
	args := buildCLIArgs(opts)

	// MCP bridge integration: add --mcp-config, --settings, --disallowedTools
	var cleanupFuncs []func()
	if m.gatewayAddr != "" {
		mcpData := providers.BuildCLIMCPConfigData(nil, m.gatewayAddr, m.gatewayToken)
		if mcpData != nil {
			mcpPath := mcpData.WriteMCPConfig(ctx, "cc-"+opts.ProjectID.String(), providers.BridgeContext{})
			if mcpPath != "" {
				args = append(args, "--mcp-config", mcpPath)
			}
		}

		hooksPath, hooksCleanup, hooksErr := providers.BuildCLIHooksConfig(workDir, true)
		if hooksErr != nil {
			slog.Warn("cc: failed to build hooks config", "error", hooksErr)
		} else if hooksPath != "" {
			args = append(args, "--settings", hooksPath)
			cleanupFuncs = append(cleanupFuncs, hooksCleanup)
		}

		// Disable CLI's built-in tools so all tool calls route through GoClaw's MCP bridge
		args = append(args, "--disallowedTools",
			"Bash,Edit,Read,Write,Glob,Grep,WebFetch,WebSearch,TodoRead,TodoWrite,NotebookRead,NotebookEdit")
	}

	// Use Background so the child process outlives the HTTP request context
	procCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, "claude", args...)
	cmd.Dir = workDir
	cmd.Env = buildEnv(opts.EnvVars)
	cmd.Stdin = strings.NewReader(prompt)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	cleanupTempFiles := func() {
		for _, fn := range cleanupFuncs {
			fn()
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		cleanupTempFiles()
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		cleanupTempFiles()
		return fmt.Errorf("start claude: %w", err)
	}

	// Update session with PID + running status
	pid := cmd.Process.Pid
	_ = m.store.UpdateSession(ctx, sessionID, map[string]any{
		"status": store.ProjectSessionStatusRunning,
		"pid":    pid,
	})

	mp := &ManagedProcess{
		cmd:       cmd,
		cancel:    cancel,
		sessionID: sessionID,
		projectID: projectID,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}

	m.mu.Lock()
	m.processes[sessionID] = mp
	m.mu.Unlock()

	// Reader goroutine: parse stream-json lines
	go func() {
		defer close(mp.done)
		defer func() {
			m.mu.Lock()
			delete(m.processes, sessionID)
			m.mu.Unlock()
			cleanupTempFiles()
		}()

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

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
				_ = m.store.UpdateSession(context.Background(), sessionID, map[string]any{
					"claude_session_id": event.SessionID,
				})
			}

			// Accumulate tokens/cost from result events
			if event.Type == "result" {
				m.accumulateResult(sessionID, event)
			}

			// Forward event to callback
			if m.eventCB != nil {
				m.eventCB(sessionID, event)
			}
		}

		// Wait for process to exit
		waitErr := cmd.Wait()
		finalStatus := store.ProjectSessionStatusCompleted
		var errStr *string
		if waitErr != nil {
			finalStatus = store.ProjectSessionStatusFailed
			s := waitErr.Error()
			if stderr := stderrBuf.String(); stderr != "" {
				s += ": " + stderr
				slog.Warn("cc: process stderr", "session_id", sessionID, "stderr", stderr)
			}
			errStr = &s
		}

		updates := map[string]any{
			"stopped_at": time.Now().UTC(),
			"pid":        nil,
		}
		// Only override status if not already set by result event
		currentSess, getErr := m.store.GetSession(context.Background(), sessionID)
		if getErr == nil && currentSess.Status == store.ProjectSessionStatusRunning {
			updates["status"] = finalStatus
			if errStr != nil {
				updates["error"] = *errStr
			}
		}
		_ = m.store.UpdateSession(context.Background(), sessionID, updates)
	}()

	return nil
}

// accumulateResult adds token/cost from a result event to the session's running totals.
func (m *ProcessManager) accumulateResult(sessionID uuid.UUID, event StreamEvent) {
	sess, err := m.store.GetSession(context.Background(), sessionID)
	if err != nil {
		slog.Warn("cc: failed to read session for token accumulation", "error", err)
		return
	}

	updates := map[string]any{
		"input_tokens":          sess.InputTokens + int64(event.InputTokens),
		"output_tokens":         sess.OutputTokens + int64(event.OutputTokens),
		"cache_read_tokens":     sess.CacheReadTokens + int64(event.CacheReadInputTokens),
		"cache_creation_tokens": sess.CacheCreationTokens + int64(event.CacheCreationTokens),
		"cost_usd":              sess.CostUSD + event.CostUSD,
	}
	if event.Subtype == "success" {
		updates["status"] = store.ProjectSessionStatusCompleted
		updates["stopped_at"] = time.Now().UTC()
	} else if event.Subtype == "error" {
		updates["status"] = store.ProjectSessionStatusFailed
		updates["stopped_at"] = time.Now().UTC()
	}
	_ = m.store.UpdateSession(context.Background(), sessionID, updates)
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
		"status":     store.ProjectSessionStatusStopped,
		"stopped_at": time.Now().UTC(),
		"pid":        nil,
	})

	slog.Info("cc: process stopped", "session_id", sessionID)
	return nil
}

// Delete stops a running session (if any) and removes it from the store.
func (m *ProcessManager) Delete(ctx context.Context, sessionID uuid.UUID) error {
	if m.IsRunning(sessionID) {
		if err := m.Stop(ctx, sessionID); err != nil {
			slog.Warn("cc: stop before delete failed", "error", err)
		}
	}
	if err := m.store.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	slog.Info("cc: session deleted", "session_id", sessionID)
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

// watchTimeout stops a session if it exceeds max duration.
func (m *ProcessManager) watchTimeout(sessionID uuid.UUID, maxDuration time.Duration) {
	m.mu.RLock()
	mp, ok := m.processes[sessionID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	select {
	case <-mp.done:
		// Process finished before timeout
	case <-time.After(maxDuration):
		if m.IsRunning(sessionID) {
			slog.Warn("cc: session exceeded max_duration, stopping", "session_id", sessionID, "max_duration", maxDuration)
			_ = m.Stop(context.Background(), sessionID)
			_ = m.store.UpdateSession(context.Background(), sessionID, map[string]any{
				"error": fmt.Sprintf("session timed out after %s", maxDuration),
			})
		}
	}
}

// SendPrompt sends a follow-up prompt to an existing session using --resume.
// Reuses the same session ID — no new session is created.
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

	opts := StartOpts{
		ProjectID:    sess.ProjectID,
		WorkDir:      proj.WorkDir,
		Prompt:       prompt,
		ResumeID:     resumeID,
		AllowedTools: allowedTools,
	}

	if err := m.spawnAndWatch(ctx, sessionID, sess.ProjectID, opts, proj.WorkDir); err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	slog.Info("cc: resumed session", "session_id", sessionID)
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
