package bridge

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/bridge.md
var bridgePrompt string

// BridgePrompt returns the embedded bridge prompt template.
func BridgePrompt() string { return bridgePrompt }

const (
	sprintTasksFile      = "sprint-tasks.md"
	sprintTasksBakSuffix = ".bak"
)

// Run converts story files to sprint-tasks.md.
// Returns (taskCount, promptLines, error). TaskCount is the number of open
// tasks (matching config.TaskOpenRegex) in the generated output.
// PromptLines is the assembled prompt line count for caller warning decisions.
//
// Deviation from AC #6 (int, error): returns (int, int, error) because AC #5
// requires bridge to return prompt line count without logging. ACs #5 and #6
// contradict — this resolves by keeping "packages don't log" mandate intact.
func Run(ctx context.Context, cfg *config.Config, storyFiles []string) (int, int, error) {
	// 2.1: Validate and read story files
	if len(storyFiles) == 0 {
		return 0, 0, fmt.Errorf("bridge: no story files provided")
	}

	var parts []string
	for _, f := range storyFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return 0, 0, fmt.Errorf("bridge: read story: %w", err)
		}
		parts = append(parts, string(data))
	}
	storyContent := strings.Join(parts, "\n\n---\n\n")

	// 2.1b: Merge detection — check for existing sprint-tasks.md
	existingPath := filepath.Join(cfg.ProjectRoot, sprintTasksFile)
	var existingData []byte    // raw bytes for backup (byte-for-byte guarantee)
	var existingContent string // string for prompt injection
	mergeMode := false
	if info, err := os.Stat(existingPath); err == nil && !info.IsDir() {
		data, readErr := os.ReadFile(existingPath)
		if readErr != nil {
			return 0, 0, fmt.Errorf("bridge: merge: read existing: %w", readErr)
		}
		existingData = data
		existingContent = string(data)
		mergeMode = true
	}

	// 2.1c: Create backup BEFORE any mutation (merge mode only)
	if mergeMode {
		bakPath := existingPath + sprintTasksBakSuffix
		if err := os.WriteFile(bakPath, existingData, 0644); err != nil {
			return 0, 0, fmt.Errorf("bridge: merge: backup: %w", err)
		}
	}

	// 2.2: Assemble prompt
	prompt, err := config.AssemblePrompt(
		bridgePrompt,
		config.TemplateData{HasExistingTasks: mergeMode},
		map[string]string{
			"__STORY_CONTENT__":   storyContent,
			"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
			"__EXISTING_TASKS__":  existingContent,
		},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("bridge: assemble prompt: %w", err)
	}

	// 2.3: Compute prompt line count (returned to caller for warning decision)
	promptLines := strings.Count(prompt, "\n") + 1

	// 2.4: Call session.Execute
	start := time.Now()
	raw, execErr := session.Execute(ctx, session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   cfg.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	})
	elapsed := time.Since(start)

	if execErr != nil {
		var debugParts []string
		if raw != nil {
			if len(raw.Stderr) > 0 {
				debugParts = append(debugParts, fmt.Sprintf("stderr: %s", raw.Stderr))
			}
			if len(raw.Stdout) > 0 {
				debugParts = append(debugParts, fmt.Sprintf("stdout: %.500s", raw.Stdout))
			}
		}
		if len(debugParts) > 0 {
			return 0, promptLines, fmt.Errorf("bridge: execute: %w\n%s", execErr, strings.Join(debugParts, "\n"))
		}
		return 0, promptLines, fmt.Errorf("bridge: execute: %w", execErr)
	}

	// 2.5: Parse result
	result, err := session.ParseResult(raw, elapsed)
	if err != nil {
		return 0, promptLines, fmt.Errorf("bridge: parse result: %w", err)
	}

	// 2.6: Count tasks
	taskCount := 0
	for _, line := range strings.Split(result.Output, "\n") {
		if config.TaskOpenRegex.MatchString(line) {
			taskCount++
		}
	}

	// 2.7: Write output (atomic: only on success)
	outPath := filepath.Join(cfg.ProjectRoot, sprintTasksFile)
	if err := os.WriteFile(outPath, []byte(result.Output), 0644); err != nil {
		return 0, promptLines, fmt.Errorf("bridge: write tasks: %w", err)
	}

	return taskCount, promptLines, nil
}
