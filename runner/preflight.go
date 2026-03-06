package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// TaskHash returns the first 6 hex characters of the SHA-256 hash of the task description.
// The description is extracted by stripping "- [ ] " or "- [x] " prefix from taskText.
// Returns lowercase hex string of length 6.
func TaskHash(taskText string) string {
	desc := strings.TrimPrefix(taskText, "- [ ] ")
	desc = strings.TrimPrefix(desc, "- [x] ")
	h := sha256.Sum256([]byte(desc))
	return hex.EncodeToString(h[:])[:6]
}

// PreFlightCheck checks if a task was already completed by searching recent git log
// for a commit with [task:<hash>] marker. If found and no review-findings.md exists,
// returns skip=true. Errors are logged but not propagated (best-effort).
//
// NOTE: no RunLogger available in standalone functions — using stdlib log.
func PreFlightCheck(ctx context.Context, git GitClient, taskText, projectRoot string) (skip bool, reason string) {
	hash := TaskHash(taskText)
	marker := fmt.Sprintf("[task:%s]", hash)

	lines, err := git.LogOneline(ctx, 20)
	if err != nil {
		log.Printf("WARN: pre-flight git log failed: %v", err)
		return false, "git log error, proceeding"
	}

	found := false
	for _, line := range lines {
		if strings.Contains(line, marker) {
			found = true
			break
		}
	}

	if !found {
		return false, fmt.Sprintf("no matching commit for %s", marker)
	}

	// Commit found — check for pending review findings
	findingsPath := filepath.Join(projectRoot, "review-findings.md")
	info, err := os.Stat(findingsPath)
	if err == nil && info.Size() > 0 {
		return false, fmt.Sprintf("commit found but findings exist (%s)", findingsPath)
	}

	return true, fmt.Sprintf("commit found, no findings (%s)", marker)
}

// SmartMergeStatus preserves [x] status from oldContent when regenerating sprint-tasks.md.
// Tasks are matched by TaskHash. Non-task lines in newContent pass through unchanged.
// Only top-level tasks (non-indented) are matched; nested subtasks are not supported.
// Returns newContent unmodified if oldContent is empty.
func SmartMergeStatus(oldContent, newContent string) string {
	if oldContent == "" {
		return newContent
	}

	// Build map of done task hashes from old content.
	doneHashes := map[string]bool{}
	for _, line := range strings.Split(oldContent, "\n") {
		if strings.HasPrefix(line, config.TaskDone+" ") {
			h := TaskHash(line)
			doneHashes[h] = true
		}
	}

	// Process new content: transfer [x] where hash matches.
	var result []string
	for _, line := range strings.Split(newContent, "\n") {
		if strings.HasPrefix(line, config.TaskOpen+" ") {
			h := TaskHash(line)
			if doneHashes[h] {
				line = strings.Replace(line, config.TaskOpen, config.TaskDone, 1)
			}
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
