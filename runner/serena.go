package runner

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

const (
	serenaMemoriesDir = ".serena/memories"
	serenaBackupDir   = ".serena/memories.bak"
)

// CodeIndexerDetector detects code indexing tools and provides prompt hints.
// Minimal interface per M5/C3: no index commands, no timeout management, no progress output.
type CodeIndexerDetector interface {
	Available(projectRoot string) bool
	PromptHint() string
}

// NoOpCodeIndexerDetector is the default when Serena detection is disabled.
type NoOpCodeIndexerDetector struct{}

// Available always returns false for NoOpCodeIndexerDetector.
func (n *NoOpCodeIndexerDetector) Available(_ string) bool { return false }

// PromptHint always returns empty string for NoOpCodeIndexerDetector.
func (n *NoOpCodeIndexerDetector) PromptHint() string { return "" }

// SerenaMCPDetector detects Serena MCP server via config file inspection.
// Detection is file-based only — no exec.LookPath, no subprocess calls (C3).
type SerenaMCPDetector struct{}

// Compile-time interface checks.
var (
	_ CodeIndexerDetector = (*SerenaMCPDetector)(nil)
	_ CodeIndexerDetector = (*NoOpCodeIndexerDetector)(nil)
)

// Available checks .claude/settings.json and .mcp.json for Serena MCP config.
// Best-effort: any read/parse error returns false.
func (s *SerenaMCPDetector) Available(projectRoot string) bool {
	// Try .claude/settings.json first
	settingsPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if hasSerenamcp(settingsPath) {
		return true
	}

	// Fallback: .mcp.json
	mcpPath := filepath.Join(projectRoot, ".mcp.json")
	return hasSerenamcp(mcpPath)
}

// PromptHint returns the Serena prompt hint for injection into execute/review prompts.
func (s *SerenaMCPDetector) PromptHint() string {
	return "If Serena MCP tools available, use them for code navigation"
}

// hasSerenamcp reads a JSON config file and checks for Serena in mcpServers keys.
func hasSerenamcp(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false
	}

	servers, ok := parsed["mcpServers"]
	if !ok {
		return false
	}

	serversMap, ok := servers.(map[string]any)
	if !ok {
		return false
	}

	for key := range serversMap {
		if strings.Contains(strings.ToLower(key), "serena") {
			return true
		}
	}
	return false
}

// serenaDetectedMsg is the stderr log message emitted when Serena MCP is detected at startup.
const serenaDetectedMsg = "Serena MCP detected"

// DetectSerena runs Serena detection at startup and logs if found.
// Returns hint string for prompt injection (empty if unavailable).
func DetectSerena(indexer CodeIndexerDetector, projectRoot string) string {
	if indexer.Available(projectRoot) {
		fmt.Fprintf(os.Stderr, "%s\n", serenaDetectedMsg)
		return indexer.PromptHint()
	}
	return ""
}

//go:embed prompts/serena-sync.md
var serenaSyncTemplate string

// SerenaSyncOpts contains inputs for assembling the sync prompt.
type SerenaSyncOpts struct {
	DiffSummary    string
	Learnings      string
	CompletedTasks string
	MaxTurns       int
	ProjectRoot    string
}

// assembleSyncPrompt builds the Serena sync prompt from template and options.
// Uses the two-stage assembly pattern: Stage 1 (template conditionals) + Stage 2 (string replacements).
func assembleSyncPrompt(opts SerenaSyncOpts) (string, error) {
	data := config.TemplateData{
		HasLearnings:      opts.Learnings != "",
		HasCompletedTasks: opts.CompletedTasks != "",
	}

	replacements := map[string]string{
		"__PROJECT_ROOT__":      opts.ProjectRoot,
		"__MAX_TURNS__":         fmt.Sprintf("%d", opts.MaxTurns),
		"__DIFF_SUMMARY__":      opts.DiffSummary,
		"__LEARNINGS_CONTENT__": opts.Learnings,
		"__COMPLETED_TASKS__":   opts.CompletedTasks,
	}

	result, err := config.AssemblePrompt(serenaSyncTemplate, data, replacements)
	if err != nil {
		return "", fmt.Errorf("runner: assemble sync prompt: %w", err)
	}
	return result, nil
}

// copyDir recursively copies src directory to dst using filepath.Walk.
// Uses os.ReadFile/os.WriteFile for NTFS compatibility (no hard links or symlinks).
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// backupMemories copies .serena/memories/ to .serena/memories.bak/.
// Removes any previous backup before copying. Returns error if source doesn't exist.
func backupMemories(projectRoot string) error {
	src := filepath.Join(projectRoot, serenaMemoriesDir)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("runner: serena sync: backup: %w", err)
	}
	dst := filepath.Join(projectRoot, serenaBackupDir)
	os.RemoveAll(dst) // clean previous backup, ignore error
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("runner: serena sync: backup: %w", err)
	}
	return nil
}

// rollbackMemories restores .serena/memories/ from .serena/memories.bak/.
// Replaces current memories entirely. Backup directory is preserved after rollback.
func rollbackMemories(projectRoot string) error {
	src := filepath.Join(projectRoot, serenaBackupDir)
	dst := filepath.Join(projectRoot, serenaMemoriesDir)
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("runner: serena sync: rollback: %w", err)
	}
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("runner: serena sync: rollback: %w", err)
	}
	return nil
}

// cleanupBackup removes .serena/memories.bak/ directory.
// Idempotent — no error if backup doesn't exist.
func cleanupBackup(projectRoot string) {
	os.RemoveAll(filepath.Join(projectRoot, serenaBackupDir))
}

// countMemoryFiles counts .md files in .serena/memories/ (top-level only, no recursion).
func countMemoryFiles(projectRoot string) (int, error) {
	dir := filepath.Join(projectRoot, serenaMemoriesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count, nil
}

// validateMemories checks that memory count hasn't decreased after sync.
// Returns nil on read error (best effort — skip validation gracefully).
func validateMemories(projectRoot string, countBefore int) error {
	countAfter, err := countMemoryFiles(projectRoot)
	if err != nil {
		return nil // best effort: skip validation on read error
	}
	if countAfter < countBefore {
		return fmt.Errorf("runner: serena sync: memory count decreased: %d → %d", countBefore, countAfter)
	}
	return nil
}

// runSerenaSync performs the full Serena memory sync flow.
// Called after each task (trigger=="task") or after execute loop (trigger=="run").
// Best-effort: all errors are logged internally, never propagated to caller.
// Flow: countMemoryFiles → backupMemories → buildSyncOpts → SerenaSyncFn → validateMemories → cleanupBackup.
// On sync error → rollback + log warning. On validation error → rollback + log warning.
// On backup error → skip sync + log warning. On count error → skip sync + log warning.
func (r *Runner) runSerenaSync(ctx context.Context, initialCommit string, taskText string) {
	log := r.logger()
	t0 := time.Now()

	// 1. Count memory files before sync
	countBefore, err := countMemoryFiles(r.Cfg.ProjectRoot)
	if err != nil {
		log.Warn("serena sync skipped", "reason", "count error", "error", err.Error())
		r.Metrics.RecordSerenaSync("skipped", time.Since(t0).Milliseconds(), nil)
		return
	}

	// 2. Backup memories
	if err := backupMemories(r.Cfg.ProjectRoot); err != nil {
		log.Warn("serena sync skipped", "reason", "backup error", "error", err.Error())
		r.Metrics.RecordSerenaSync("skipped", time.Since(t0).Milliseconds(), nil)
		return
	}

	// 3. Build sync options
	opts := r.buildSyncOpts(ctx, initialCommit, taskText)

	// 4. Run sync session
	result, err := r.SerenaSyncFn(ctx, opts)
	if err != nil {
		log.Warn("serena sync failed, rolling back", "error", err.Error())
		if rbErr := rollbackMemories(r.Cfg.ProjectRoot); rbErr != nil {
			log.Warn("serena sync rollback failed", "error", rbErr.Error())
		}
		r.Metrics.RecordSerenaSync("failed", time.Since(t0).Milliseconds(), result)
		return
	}

	// 5. Validate memories
	if err := validateMemories(r.Cfg.ProjectRoot, countBefore); err != nil {
		log.Warn("serena sync validation failed, rolling back", "error", err.Error())
		if rbErr := rollbackMemories(r.Cfg.ProjectRoot); rbErr != nil {
			log.Warn("serena sync rollback failed", "error", rbErr.Error())
		}
		r.Metrics.RecordSerenaSync("rollback", time.Since(t0).Milliseconds(), result)
		return
	}

	// 6. Cleanup backup on success
	cleanupBackup(r.Cfg.ProjectRoot)
	log.Info("serena sync completed")
	r.Metrics.RecordSerenaSync("success", time.Since(t0).Milliseconds(), result)
}

// extractCompletedTasks reads tasksFile and returns only lines matching completed tasks ([x]).
// Returns empty string on read error (best effort).
func extractCompletedTasks(tasksFile string) string {
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return ""
	}
	var completed []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, config.TaskDone) {
			completed = append(completed, line)
		}
	}
	return strings.Join(completed, "\n")
}

// buildSyncOpts gathers context for the Serena sync prompt.
// DiffSummary from git diff stats (initialCommit..HEAD), Learnings from LEARNINGS.md.
// CompletedTasks: when taskText is non-empty (per-task mode), uses taskText directly;
// when empty (batch mode), extracts completed lines from sprint-tasks.md.
func (r *Runner) buildSyncOpts(ctx context.Context, initialCommit string, taskText string) SerenaSyncOpts {
	opts := SerenaSyncOpts{
		MaxTurns:    r.Cfg.SerenaSyncMaxTurns,
		ProjectRoot: r.Cfg.ProjectRoot,
	}

	// DiffSummary: format git diff stats as text
	if initialCommit != "" {
		if ds, err := r.Git.DiffStats(ctx, initialCommit, "HEAD"); err == nil {
			opts.DiffSummary = fmt.Sprintf("%d files changed, +%d/-%d", ds.FilesChanged, ds.Insertions, ds.Deletions)
		}
	}

	// Learnings: read LEARNINGS.md, empty on error
	learningsPath := filepath.Join(r.Cfg.ProjectRoot, "LEARNINGS.md")
	if data, err := os.ReadFile(learningsPath); err == nil {
		opts.Learnings = string(data)
	}

	// CompletedTasks: per-task uses taskText directly, batch extracts from file
	if taskText != "" {
		opts.CompletedTasks = taskText
	} else {
		opts.CompletedTasks = extractCompletedTasks(r.TasksFile)
	}

	return opts
}

// RealSerenaSync executes a Serena memory sync session via Claude CLI.
// Assembles sync prompt from opts, runs session.Execute, and parses the result.
// Returns error wrapped with "runner: serena sync:" prefix on any failure.
func RealSerenaSync(ctx context.Context, cfg *config.Config, opts SerenaSyncOpts, logger *RunLogger) (*session.SessionResult, error) {
	prompt, err := assembleSyncPrompt(opts)
	if err != nil {
		return nil, fmt.Errorf("runner: serena sync: %w", err)
	}

	// Story 10.6 AC8: compaction counter for sync session.
	syncCounterPath, syncCounterCleanup := CreateCompactCounter()
	defer syncCounterCleanup()

	sessOpts := session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        opts.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   opts.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	}
	if syncCounterPath != "" {
		sessOpts.Env = map[string]string{"RALPH_COMPACT_COUNTER": syncCounterPath}
	}
	t0 := time.Now()
	raw, err := session.Execute(ctx, sessOpts)
	elapsed := time.Since(t0)
	if logger != nil {
		logger.SaveSession("sync", raw, raw.ExitCode, elapsed)
	}
	if err != nil {
		return nil, fmt.Errorf("runner: serena sync: %w", err)
	}

	result, err := session.ParseResult(raw, elapsed)
	if err != nil {
		return nil, fmt.Errorf("runner: serena sync: parse: %w", err)
	}
	if result.ExitCode != 0 {
		return result, fmt.Errorf("runner: serena sync: session exit code %d", result.ExitCode)
	}
	return result, nil
}
