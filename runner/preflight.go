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
