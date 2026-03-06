package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateLearnings performs JIT citation validation on LEARNINGS.md content.
// Entries with citations referencing non-existent files are classified as stale.
// Valid entries are returned in reverse order (recency-first, L3).
// Stale entries are returned separately for removal at distillation.
// Validation is os.Stat file existence check only — no line range validation (M9).
func ValidateLearnings(projectRoot, content string) (valid string, stale string) {
	if strings.TrimSpace(content) == "" {
		return "", ""
	}

	sections := splitSections(content)
	if len(sections) == 0 {
		return "", ""
	}

	var validSections []string
	var staleSections []string

	for _, sec := range sections {
		file := extractCitationFile(sec)
		if file == "" {
			// No citation found — treat as valid (no file to check)
			validSections = append(validSections, sec)
			continue
		}
		fullPath := filepath.Join(projectRoot, file)
		if _, err := os.Stat(fullPath); err != nil {
			staleSections = append(staleSections, sec)
		} else {
			validSections = append(validSections, sec)
		}
	}

	// Reverse order for recency-first injection (L3)
	for i, j := 0, len(validSections)-1; i < j; i, j = i+1, j-1 {
		validSections[i], validSections[j] = validSections[j], validSections[i]
	}

	validOut := joinSections(validSections)
	staleOut := joinSections(staleSections)

	return validOut, staleOut
}

// splitSections splits LEARNINGS.md content by "\n## " headers into sections.
// The first section (before any ## header) is preserved if non-empty.
func splitSections(content string) []string {
	parts := strings.Split(content, "\n## ")
	var sections []string
	for i, p := range parts {
		if i == 0 {
			// First part: content before first ## header (or starting with ##)
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				sections = append(sections, trimmed)
			}
		} else {
			sections = append(sections, "## "+p)
		}
	}
	return sections
}

// joinSections joins sections back into a single string with single newline separator.
// Sections already carry their "## " prefix, so "\n" preserves original formatting.
func joinSections(sections []string) string {
	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n")
}

// citationFileRegex extracts file path from citation like [source, file.go:42]
// Uses the existing citationRegex from knowledge_write.go for format validation,
// but extracts the file path portion specifically.
func extractCitationFile(section string) string {
	// Find first line starting with "## "
	lines := strings.Split(section, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		// Extract citation: text between last [ and ]
		lastOpen := strings.LastIndex(line, "[")
		lastClose := strings.LastIndex(line, "]")
		if lastOpen < 0 || lastClose <= lastOpen {
			return ""
		}
		citation := line[lastOpen+1 : lastClose]
		// Citation format: "source, file:line" — extract file part
		parts := strings.Split(citation, ",")
		if len(parts) < 2 {
			return ""
		}
		fileRef := strings.TrimSpace(parts[len(parts)-1])
		// Strip line number: "file.go:42" → "file.go"
		if colonIdx := strings.LastIndex(fileRef, ":"); colonIdx > 0 {
			fileRef = fileRef[:colonIdx]
		}
		return fileRef
	}
	return ""
}

// buildKnowledgeReplacements reads knowledge files and returns Stage 2 replacement map.
// Returns:
//   - map with __LEARNINGS_CONTENT__ and __RALPH_KNOWLEDGE__ keys
//   - *string for --append-system-prompt content (nil if no ralph-critical.md)
//   - error on unexpected file read failures
//
// Missing files result in empty strings (no error).
// 0 files written to .claude/ directory.
func buildKnowledgeReplacements(projectRoot string) (map[string]string, *string, error) {
	replacements := map[string]string{
		"__LEARNINGS_CONTENT__": "",
		"__RALPH_KNOWLEDGE__":   "",
	}

	// Read LEARNINGS.md → ValidateLearnings → inject validated content
	learningsPath := filepath.Join(projectRoot, "LEARNINGS.md")
	learningsData, err := os.ReadFile(learningsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("runner: build knowledge: read learnings: %w", err)
	}
	if err == nil {
		valid, _ := ValidateLearnings(projectRoot, string(learningsData))
		replacements["__LEARNINGS_CONTENT__"] = valid
	}

	// Read .ralph/rules/ralph-*.md files
	rulesDir := filepath.Join(projectRoot, ".ralph", "rules")
	pattern := filepath.Join(rulesDir, "ralph-*.md")
	matches, globErr := filepath.Glob(pattern)
	if globErr != nil {
		return nil, nil, fmt.Errorf("runner: build knowledge: glob rules: %w", globErr)
	}

	var appendSystemPrompt *string
	var channelTwoContent []string

	for _, match := range matches {
		base := filepath.Base(match)
		// Exclude ralph-index.md
		if base == "ralph-index.md" {
			continue
		}

		data, readErr := os.ReadFile(match)
		if readErr != nil {
			return nil, nil, fmt.Errorf("runner: build knowledge: read rule %s: %w", base, readErr)
		}
		content := string(data)

		if base == "ralph-critical.md" {
			// Channel 1: system prompt delivery
			appendSystemPrompt = &content
		} else {
			// Channel 2: user prompt delivery
			channelTwoContent = append(channelTwoContent, content)
		}
	}

	if len(channelTwoContent) > 0 {
		replacements["__RALPH_KNOWLEDGE__"] = strings.Join(channelTwoContent, "\n\n---\n\n")
	}

	return replacements, appendSystemPrompt, nil
}
