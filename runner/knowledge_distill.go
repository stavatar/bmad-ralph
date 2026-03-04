package runner

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/distill.md
var distillTemplate string

// languageGlobs maps file extensions to relevant glob patterns for scope detection.
var languageGlobs = map[string][]string{
	".go":   {"**/*.go", "**/*_test.go"},
	".ts":   {"**/*.ts", "**/*.tsx"},
	".js":   {"**/*.js", "**/*.jsx"},
	".py":   {"**/*.py"},
	".rs":   {"**/*.rs"},
	".java": {"**/*.java"},
	".rb":   {"**/*.rb"},
	".php":  {"**/*.php"},
	".c":    {"**/*.c", "**/*.h"},
	".cpp":  {"**/*.cpp", "**/*.hpp"},
}

// beginMarker and endMarker delimit distillation output (H6).
const (
	beginMarker = "BEGIN_DISTILLED_OUTPUT"
	endMarker   = "END_DISTILLED_OUTPUT"
)

// categoryHeaderRegex matches ## CATEGORY: <name> lines.
var categoryHeaderRegex = regexp.MustCompile(`^## CATEGORY:\s+(\S+)`)

// entryHeaderRegex matches ## <category>: <topic> [citation] [freq:N] [stage:<stage>] lines.
var entryHeaderRegex = regexp.MustCompile(`^## (\w[\w-]*):\s+(.+?)\s+\[(.+?)\]`)

// freqRegex extracts [freq:N] from entry header.
var freqRegex = regexp.MustCompile(`\[freq:(\d+)\]`)

// stageRegex extracts [stage:<value>] from entry header.
var stageRegex = regexp.MustCompile(`\[stage:(execute|review|both)\]`)

// newCategoryRegex matches NEW_CATEGORY: <name> markers.
var newCategoryRegex = regexp.MustCompile(`^NEW_CATEGORY:\s+(\S+)`)

// DistilledEntry represents a single parsed entry from distillation output.
type DistilledEntry struct {
	Content  string // entry text (header + body)
	Freq     int    // [freq:N] value (0 = not set)
	Stage    string // execute, review, both
	IsAnchor bool   // true if ANCHOR marker present
}

// DistillOutput holds parsed distillation results.
type DistillOutput struct {
	CompressedLearnings string                      // raw content for LEARNINGS.md
	Categories          map[string][]DistilledEntry // category → entries
	NewCategories       []string                    // NEW_CATEGORY proposals
}

// DetectProjectScope scans top 2 levels of project tree for file extensions
// and maps them to known language globs (M4).
// Returns formatted string: "Project languages detected. Relevant globs: **/*.go, **/*_test.go"
// Returns "No language-specific patterns detected" if no known extensions found.
func DetectProjectScope(projectRoot string) (string, error) {
	extensions := make(map[string]bool)

	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}

		rel, relErr := filepath.Rel(projectRoot, path)
		if relErr != nil {
			return nil
		}

		// Depth check: count separators for top 2 levels
		depth := strings.Count(rel, string(filepath.Separator))
		if d.IsDir() {
			if depth >= 2 {
				return filepath.SkipDir
			}
			// Skip hidden directories
			if strings.HasPrefix(d.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext != "" {
			extensions[ext] = true
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("runner: distill: scope: %w", err)
	}

	var globs []string
	seen := make(map[string]bool)
	for ext := range extensions {
		if patterns, ok := languageGlobs[ext]; ok {
			for _, p := range patterns {
				if !seen[p] {
					globs = append(globs, p)
					seen[p] = true
				}
			}
		}
	}

	if len(globs) == 0 {
		return "No language-specific patterns detected", nil
	}

	return "Project languages detected. Relevant globs: " + strings.Join(globs, ", "), nil
}

// BackupFile performs 2-generation backup rotation: .bak.1 ← .bak ← current.
// Missing source file is a no-op (returns nil).
func BackupFile(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	bakPath := path + ".bak"
	bak1Path := path + ".bak.1"

	// Rotate: .bak → .bak.1
	if _, err := os.Stat(bakPath); err == nil {
		if err := os.Rename(bakPath, bak1Path); err != nil {
			return fmt.Errorf("runner: backup: %w", err)
		}
	}

	// Current → .bak
	if err := os.Rename(path, bakPath); err != nil {
		return fmt.Errorf("runner: backup: %w", err)
	}

	return nil
}

// BackupDistillationFiles backs up LEARNINGS.md, all ralph-*.md, and distill-state.json
// with 2-generation rotation.
func BackupDistillationFiles(projectRoot string) error {
	learningsPath := filepath.Join(projectRoot, "LEARNINGS.md")
	if err := BackupFile(learningsPath); err != nil {
		return fmt.Errorf("runner: distill: backup: %w", err)
	}

	rulesDir := filepath.Join(projectRoot, ".ralph", "rules")
	matches, globErr := filepath.Glob(filepath.Join(rulesDir, "ralph-*.md"))
	if globErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: distill: backup: glob rules: %v\n", globErr)
	}
	for _, m := range matches {
		if err := BackupFile(m); err != nil {
			return fmt.Errorf("runner: distill: backup: %w", err)
		}
	}

	statePath := filepath.Join(projectRoot, ".ralph", "distill-state.json")
	if err := BackupFile(statePath); err != nil {
		return fmt.Errorf("runner: distill: backup: %w", err)
	}

	return nil
}

// RestoreDistillationBackups restores LEARNINGS.md, ralph-*.md, and distill-state.json
// from .bak files. Used by `ralph distill` abort path.
// Missing .bak files are skipped (file wasn't backed up originally).
// RestoreDistillationBackups reverses BackupDistillationFiles: renames .bak → original
// for LEARNINGS.md, ralph-*.md, and distill-state.json.
// Missing .bak files are skipped (idempotent).
// Error wrapping: "runner: distill: restore:" prefix.
func RestoreDistillationBackups(projectRoot string) error {
	learningsPath := filepath.Join(projectRoot, "LEARNINGS.md")
	if err := restoreFromBackup(learningsPath); err != nil {
		return fmt.Errorf("runner: distill: restore: %w", err)
	}

	rulesDir := filepath.Join(projectRoot, ".ralph", "rules")
	// Restore from .bak files (original names)
	bakMatches, bakGlobErr := filepath.Glob(filepath.Join(rulesDir, "ralph-*.md.bak"))
	if bakGlobErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: distill: restore: glob rules: %v\n", bakGlobErr)
	}
	for _, bak := range bakMatches {
		target := strings.TrimSuffix(bak, ".bak")
		if err := os.Rename(bak, target); err != nil {
			return fmt.Errorf("runner: distill: restore: %w", err)
		}
	}

	statePath := filepath.Join(projectRoot, ".ralph", "distill-state.json")
	if err := restoreFromBackup(statePath); err != nil {
		return fmt.Errorf("runner: distill: restore: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Backups restored\n")
	return nil
}

// restoreFromBackup renames path.bak → path. Missing .bak is a no-op.
func restoreFromBackup(path string) error {
	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := os.Rename(bakPath, path); err != nil {
		return err
	}
	return nil
}

// ParseDistillOutput parses raw Claude output between BEGIN/END markers (H6).
// Returns ErrBadFormat if markers are missing or content is empty.
func ParseDistillOutput(raw string) (*DistillOutput, error) {
	beginIdx := strings.Index(raw, beginMarker)
	endIdx := strings.Index(raw, endMarker)

	if beginIdx < 0 || endIdx < 0 || endIdx <= beginIdx {
		return nil, fmt.Errorf("runner: distill: parse: %w", ErrBadFormat)
	}

	content := strings.TrimSpace(raw[beginIdx+len(beginMarker) : endIdx])
	if content == "" {
		return nil, fmt.Errorf("runner: distill: parse: empty content: %w", ErrBadFormat)
	}

	output := &DistillOutput{
		CompressedLearnings: content,
		Categories:          make(map[string][]DistilledEntry),
	}

	lines := strings.Split(content, "\n")
	var currentCategory string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for NEW_CATEGORY marker
		if matches := newCategoryRegex.FindStringSubmatch(trimmed); matches != nil {
			output.NewCategories = append(output.NewCategories, matches[1])
			continue
		}

		// Check for category header
		if matches := categoryHeaderRegex.FindStringSubmatch(trimmed); matches != nil {
			currentCategory = matches[1]
			continue
		}

		// Check for entry header
		if entryHeaderRegex.MatchString(trimmed) && currentCategory != "" {
			entry := parseDistilledEntry(trimmed)
			output.Categories[currentCategory] = append(output.Categories[currentCategory], entry)
			continue
		}

		// Content line — append to last entry in current category
		if currentCategory != "" && trimmed != "" {
			entries := output.Categories[currentCategory]
			if len(entries) > 0 {
				last := &output.Categories[currentCategory][len(entries)-1]
				last.Content += "\n" + trimmed
			}
		}
	}

	return output, nil
}

// parseDistilledEntry extracts DistilledEntry fields from an entry header line.
func parseDistilledEntry(line string) DistilledEntry {
	entry := DistilledEntry{
		Content:  line,
		IsAnchor: strings.Contains(line, "ANCHOR"),
	}

	if matches := freqRegex.FindStringSubmatch(line); matches != nil {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			entry.Freq = n
		}
	}

	if matches := stageRegex.FindStringSubmatch(line); matches != nil {
		entry.Stage = matches[1]
	}

	return entry
}

// ValidateFreqMonotonicity checks that new freq values are >= old values for same entries.
// Corrects Claude's arithmetic errors silently (M11).
func ValidateFreqMonotonicity(output *DistillOutput, oldFreqs map[string]int) {
	for cat := range output.Categories {
		for i := range output.Categories[cat] {
			key := cat + ":" + entryTopicKey(output.Categories[cat][i].Content)
			if oldFreq, ok := oldFreqs[key]; ok && output.Categories[cat][i].Freq < oldFreq {
				output.Categories[cat][i].Freq = oldFreq
				// Update content line with corrected freq
				output.Categories[cat][i].Content = freqRegex.ReplaceAllString(
					output.Categories[cat][i].Content, fmt.Sprintf("[freq:%d]", oldFreq))
			}
		}
	}
}

// entryTopicKey extracts the "category: topic" portion from an entry header for dedup.
func entryTopicKey(content string) string {
	firstLine := strings.SplitN(content, "\n", 2)[0]
	if matches := entryHeaderRegex.FindStringSubmatch(firstLine); matches != nil {
		return matches[2]
	}
	return firstLine
}

// WriteDistillOutput writes distillation results to multi-file structure as .pending files.
// Returns the list of target file paths (without .pending suffix) for CommitPendingFiles.
// Categories with >= 5 entries get their own ralph-{category}.md file.
// Categories with < 5 entries merge into ralph-misc.md.
// Entries with freq >= 10 promoted to ralph-critical.md with ANCHOR marker.
func WriteDistillOutput(projectRoot string, output *DistillOutput, state *DistillState, scopeHints string) ([]string, error) {
	rulesDir := filepath.Join(projectRoot, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, fmt.Errorf("runner: distill: write: %w", err)
	}

	var targetFiles []string

	// Write compressed LEARNINGS.md.pending
	learningsPath := filepath.Join(projectRoot, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath+".pending", []byte(output.CompressedLearnings+"\n"), 0644); err != nil {
		return nil, fmt.Errorf("runner: distill: write: %w", err)
	}
	targetFiles = append(targetFiles, learningsPath)

	// Collect critical entries (freq >= 10) and categorize
	var criticalEntries []DistilledEntry
	miscEntries := make(map[string][]DistilledEntry)

	for cat, entries := range output.Categories {
		var normalEntries []DistilledEntry
		for _, e := range entries {
			if e.Freq >= 10 {
				// Ensure ANCHOR marker
				if !e.IsAnchor {
					e.IsAnchor = true
					if !strings.Contains(e.Content, "ANCHOR") {
						firstLine := strings.SplitN(e.Content, "\n", 2)[0]
						rest := ""
						if idx := strings.Index(e.Content, "\n"); idx >= 0 {
							rest = e.Content[idx:]
						}
						e.Content = firstLine + " ANCHOR" + rest
					}
				}
				criticalEntries = append(criticalEntries, e)
				// Replace in source category with reference
				ref := DistilledEntry{
					Content: fmt.Sprintf("→ Promoted to ralph-critical.md (freq:%d)", e.Freq),
					Freq:    e.Freq,
					Stage:   e.Stage,
				}
				normalEntries = append(normalEntries, ref)
			} else {
				normalEntries = append(normalEntries, e)
			}
		}

		if len(normalEntries) >= 5 {
			// Write dedicated category file as .pending
			path, err := writeCategoryFilePending(rulesDir, cat, normalEntries, scopeHints)
			if err != nil {
				return nil, err
			}
			targetFiles = append(targetFiles, path)
		} else {
			// Merge into misc
			miscEntries[cat] = normalEntries
		}
	}

	// Write ralph-critical.md.pending if there are critical entries
	if len(criticalEntries) > 0 {
		path, err := writeCriticalFilePending(rulesDir, criticalEntries)
		if err != nil {
			return nil, err
		}
		targetFiles = append(targetFiles, path)
	}

	// Write ralph-misc.md.pending if there are misc entries
	if len(miscEntries) > 0 {
		path, err := writeMiscFilePending(rulesDir, miscEntries)
		if err != nil {
			return nil, err
		}
		targetFiles = append(targetFiles, path)
	}

	// Update DistillState categories
	for _, newCat := range output.NewCategories {
		if !containsString(state.Categories, newCat) {
			state.Categories = append(state.Categories, newCat)
		}
	}

	// Count total entries and categories for log
	totalEntries := 0
	for _, entries := range output.Categories {
		totalEntries += len(entries)
	}
	fmt.Fprintf(os.Stderr, "Auto-distilled LEARNINGS.md (%d categories, %d entries)\n",
		len(output.Categories), totalEntries)

	return targetFiles, nil
}

// writeCategoryFilePending writes ralph-{category}.md.pending and returns the target path (without .pending).
func writeCategoryFilePending(rulesDir, category string, entries []DistilledEntry, scopeHints string) (string, error) {
	var sb strings.Builder

	// YAML frontmatter with globs from scope hints
	globs := extractGlobsForCategory(category, scopeHints)
	// Filter valid globs first to avoid malformed separators
	var validGlobs []string
	for _, g := range globs {
		if _, err := filepath.Match(g, ""); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: invalid glob %q for category %s, skipping\n", g, category)
			continue
		}
		validGlobs = append(validGlobs, g)
	}
	if len(validGlobs) > 0 {
		sb.WriteString("---\nglobs: [")
		for i, g := range validGlobs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(g)
		}
		sb.WriteString("]\n---\n")
	}

	sb.WriteString("# " + category + "\n\n")
	for _, e := range entries {
		sb.WriteString(e.Content + "\n\n")
	}

	path := filepath.Join(rulesDir, "ralph-"+category+".md")
	if err := os.WriteFile(path+".pending", []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("runner: distill: write: %w", err)
	}
	return path, nil
}

// writeCriticalFilePending writes ralph-critical.md.pending and returns the target path.
func writeCriticalFilePending(rulesDir string, entries []DistilledEntry) (string, error) {
	var sb strings.Builder
	sb.WriteString("---\nglobs: [\"**\"]\n---\n")
	sb.WriteString("# Critical Rules (T1)\n\n")

	for _, e := range entries {
		sb.WriteString(e.Content + "\n\n")
	}

	path := filepath.Join(rulesDir, "ralph-critical.md")
	if err := os.WriteFile(path+".pending", []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("runner: distill: write: %w", err)
	}
	return path, nil
}

// writeMiscFilePending writes ralph-misc.md.pending and returns the target path.
func writeMiscFilePending(rulesDir string, categorized map[string][]DistilledEntry) (string, error) {
	var sb strings.Builder
	sb.WriteString("# Miscellaneous Rules\n\n")

	for cat, entries := range categorized {
		sb.WriteString("## " + cat + "\n\n")
		for _, e := range entries {
			sb.WriteString(e.Content + "\n\n")
		}
	}

	path := filepath.Join(rulesDir, "ralph-misc.md")
	if err := os.WriteFile(path+".pending", []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("runner: distill: write: %w", err)
	}
	return path, nil
}

// extractGlobsForCategory returns appropriate globs based on category and scope hints.
func extractGlobsForCategory(category, scopeHints string) []string {
	// For now, extract globs from scope hints string
	// Pattern: "Relevant globs: **/*.go, **/*_test.go"
	if idx := strings.Index(scopeHints, "Relevant globs: "); idx >= 0 {
		globStr := scopeHints[idx+len("Relevant globs: "):]
		parts := strings.Split(globStr, ", ")
		var globs []string
		for _, p := range parts {
			g := strings.TrimSpace(p)
			if g != "" {
				globs = append(globs, g)
			}
		}
		return globs
	}
	return nil
}

// WriteDistillIndex generates ralph-index.md with a markdown table of all ralph-*.md files.
func WriteDistillIndex(projectRoot string) error {
	rulesDir := filepath.Join(projectRoot, ".ralph", "rules")
	matches, err := filepath.Glob(filepath.Join(rulesDir, "ralph-*.md"))
	if err != nil {
		return fmt.Errorf("runner: distill: index: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Ralph Rules Index\n\n")
	sb.WriteString("| File | Entries | Globs | Last Updated |\n")
	sb.WriteString("|------|---------|-------|--------------|\n")

	for _, m := range matches {
		name := filepath.Base(m)
		if name == "ralph-index.md" {
			continue
		}

		content, readErr := os.ReadFile(m)
		if readErr != nil {
			continue
		}

		entries := strings.Count(string(content), "\n## ")
		globs := extractFrontmatterGlobs(string(content))
		info, statErr := os.Stat(m)
		lastUpdated := "unknown"
		if statErr == nil {
			lastUpdated = info.ModTime().Format("2006-01-02")
		}

		sb.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", name, entries, globs, lastUpdated))
	}

	indexPath := filepath.Join(rulesDir, "ralph-index.md")
	if err := os.WriteFile(indexPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("runner: distill: index: %w", err)
	}
	return nil
}

// extractFrontmatterGlobs extracts globs value from YAML frontmatter.
func extractFrontmatterGlobs(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return "-"
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx < 0 {
		return "-"
	}
	frontmatter := content[4 : 4+endIdx]
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "globs:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "globs:"))
		}
	}
	return "-"
}

// intentFileName is the fixed name for the distillation intent file.
const intentFileName = "distill-intent.json"

// DistillIntent records in-flight distillation for crash recovery (CR1).
// Written to {projectRoot}/.ralph/distill-intent.json.
type DistillIntent struct {
	Timestamp string   `json:"timestamp"`
	Files     []string `json:"files"`
	Phase     string   `json:"phase"` // backup, write, commit
}

// WriteIntentFile writes the intent file for atomic multi-file distillation.
func WriteIntentFile(projectRoot string, intent *DistillIntent) error {
	dir := filepath.Join(projectRoot, ".ralph")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("runner: distill: intent: %w", err)
	}
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return fmt.Errorf("runner: distill: intent: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(dir, intentFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("runner: distill: intent: %w", err)
	}
	return nil
}

// ReadIntentFile reads the distillation intent file.
// Returns nil, nil if file does not exist (normal state, no recovery needed).
func ReadIntentFile(projectRoot string) (*DistillIntent, error) {
	path := filepath.Join(projectRoot, ".ralph", intentFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("runner: distill: intent: %w", err)
	}
	var intent DistillIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("runner: distill: intent: %w", err)
	}
	return &intent, nil
}

// DeleteIntentFile removes the distillation intent file.
// Returns nil if file does not exist (idempotent).
func DeleteIntentFile(projectRoot string) error {
	path := filepath.Join(projectRoot, ".ralph", intentFileName)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("runner: distill: intent: %w", err)
	}
	return nil
}

// CommitPendingFiles renames .pending files to their target paths.
// Skips missing .pending files (idempotent for crash recovery).
// If a rename fails, remaining files are left as .pending (recoverable).
func CommitPendingFiles(files []string) error {
	for _, target := range files {
		pending := target + ".pending"
		if err := os.Rename(pending, target); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // already committed or never written
			}
			return fmt.Errorf("runner: distill: commit: %w", err)
		}
	}
	return nil
}

// ValidateDistillation checks 3 criteria on distillation output (v6 simplified):
//  1. Budget guard: total lines in CompressedLearnings <= budgetLimit
//  2. Citation preservation >= 80%: citations from oldContent preserved in new output
//  3. [needs-formatting] handling: none silently dropped (each must be fixed or still present)
//
// Returns nil if all pass. Returns ErrValidationFailed-wrapped error with criterion number on failure.
func ValidateDistillation(output *DistillOutput, oldContent string, budgetLimit int) error {
	// Criterion 1: Budget guard
	lines := strings.Count(output.CompressedLearnings, "\n") + 1
	if lines > budgetLimit {
		return fmt.Errorf("runner: distill: validate: criterion 1: budget exceeded (%d > %d lines): %w",
			lines, budgetLimit, ErrValidationFailed)
	}

	// Criterion 2: Citation preservation >= 80%
	oldCitations := extractCitations(oldContent)
	if len(oldCitations) > 0 {
		preserved := 0
		newContent := output.CompressedLearnings
		for _, c := range oldCitations {
			if strings.Contains(newContent, c) {
				preserved++
			}
		}
		pct := float64(preserved) / float64(len(oldCitations)) * 100
		if pct < 80 {
			return fmt.Errorf("runner: distill: validate: criterion 2: citation preservation %.0f%% < 80%%: %w",
				pct, ErrValidationFailed)
		}
	}

	// Criterion 3: [needs-formatting] handling — none silently dropped
	oldNF := strings.Count(oldContent, needsFormattingTag)
	if oldNF > 0 {
		// Each old [needs-formatting] entry must appear in new output (by topic):
		// either still tagged (preserved) or tag removed (fixed). Silently dropped = failure.
		oldNFTopics := extractNeedsFormattingTopics(oldContent)
		dropped := 0
		for _, topic := range oldNFTopics {
			if !strings.Contains(output.CompressedLearnings, topic) {
				dropped++
			}
		}
		if dropped > 0 {
			return fmt.Errorf("runner: distill: validate: criterion 3: %d [needs-formatting] entries silently dropped: %w",
				dropped, ErrValidationFailed)
		}
	}

	return nil
}

// extractCitations returns all citation strings from LEARNINGS.md content.
// Citations match the pattern [source, file:line].
func extractCitations(content string) []string {
	matches := citationRegex.FindAllString(content, -1)
	return matches
}

// extractNeedsFormattingTopics returns topic identifiers for entries tagged [needs-formatting].
// Topic = the "category: topic" portion from "## category: topic [citation] [needs-formatting]".
func extractNeedsFormattingTopics(content string) []string {
	var topics []string
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needsFormattingTag) && strings.HasPrefix(line, "## ") {
			// Extract topic portion: everything between "## " and first " ["
			trimmed := strings.TrimPrefix(line, "## ")
			if idx := strings.Index(trimmed, " ["); idx >= 0 {
				topics = append(topics, trimmed[:idx])
			}
		}
	}
	return topics
}

// ComputeDistillMetrics computes effectiveness metrics comparing old content to new output (AC #7).
// All fields populated: EntriesBefore/After, StaleRemoved, Categories, NeedsFormatting, T1.
func ComputeDistillMetrics(oldContent string, output *DistillOutput) *DistillMetrics {
	// EntriesBefore: count ## headers in old content
	entriesBefore := 0
	for _, line := range strings.Split(oldContent, "\n") {
		if strings.HasPrefix(line, "## ") {
			entriesBefore++
		}
	}

	// EntriesAfter: total entries across all categories
	entriesAfter := 0
	for _, entries := range output.Categories {
		entriesAfter += len(entries)
	}

	// StaleRemoved
	staleRemoved := entriesBefore - entriesAfter
	if staleRemoved < 0 {
		staleRemoved = 0
	}

	// CategoriesPreserved/Total
	categoriesTotal := len(output.Categories)
	categoriesPreserved := categoriesTotal // all output categories are preserved by definition

	// NeedsFormattingFixed: count in old minus count in new
	oldNF := strings.Count(oldContent, needsFormattingTag)
	newNF := strings.Count(output.CompressedLearnings, needsFormattingTag)
	needsFormattingFixed := oldNF - newNF
	if needsFormattingFixed < 0 {
		needsFormattingFixed = 0
	}

	// T1Promotions: entries with Freq >= 10
	t1Promotions := 0
	for _, entries := range output.Categories {
		for _, e := range entries {
			if e.Freq >= 10 {
				t1Promotions++
			}
		}
	}

	return &DistillMetrics{
		EntriesBefore:        entriesBefore,
		EntriesAfter:         entriesAfter,
		StaleRemoved:         staleRemoved,
		CategoriesPreserved:  categoriesPreserved,
		CategoriesTotal:      categoriesTotal,
		NeedsFormattingFixed: needsFormattingFixed,
		T1Promotions:         t1Promotions,
		LastDistillTime:      time.Now().UTC().Format(time.RFC3339),
	}
}

// AutoDistill is the production distillation function.
// Replaces the stub from Story 6.5a.
// Pipeline: backup → read → scope → prompt → session → parse → validate → write → metrics → index.
func AutoDistill(ctx context.Context, cfg *config.Config, state *DistillState) error {
	// Step 1: Read LEARNINGS.md content (before backup moves the file)
	learningsPath := filepath.Join(cfg.ProjectRoot, "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err != nil {
		return fmt.Errorf("runner: distill: read learnings: %w", err)
	}

	// Step 2: Backup all distillation files (after reading, for rollback safety)
	if err := BackupDistillationFiles(cfg.ProjectRoot); err != nil {
		return err // already wrapped with "runner: distill: backup:" prefix
	}

	// Step 3: Read existing ralph-*.md content
	existingRules := readExistingRules(cfg.ProjectRoot)

	// Step 4: Detect project scope
	scopeHints, scopeErr := DetectProjectScope(cfg.ProjectRoot)
	if scopeErr != nil {
		scopeHints = "No language-specific patterns detected"
	}

	// Step 5: Assemble distillation prompt (via AssemblePrompt for placeholder validation)
	prompt, promptErr := config.AssemblePrompt(
		distillTemplate,
		config.TemplateData{},
		map[string]string{
			"__LEARNINGS_CONTENT__": string(learningsContent),
			"__SCOPE_HINTS__":      scopeHints,
			"__EXISTING_RULES__":   existingRules,
		},
	)
	if promptErr != nil {
		return fmt.Errorf("runner: distill: assemble prompt: %w", promptErr)
	}

	// Step 6: Execute with timeout (H8)
	timeout := time.Duration(cfg.DistillTimeout) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   1,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	}

	raw, execErr := session.Execute(timeoutCtx, opts)
	if execErr != nil {
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("runner: distill: execute: timeout after %ds: %w", cfg.DistillTimeout, execErr)
		}
		return fmt.Errorf("runner: distill: execute: %w", execErr)
	}

	// Step 7: Parse output
	output, parseErr := ParseDistillOutput(string(raw.Stdout))
	if parseErr != nil {
		return parseErr
	}

	// Step 8: Validate distillation output (3 criteria)
	if err := ValidateDistillation(output, string(learningsContent), cfg.LearningsBudget); err != nil {
		return err // already wrapped with "runner: distill: validate:" prefix
	}

	// Step 9: Phase "write" — write .pending files + intent
	targetFiles, writeErr := WriteDistillOutput(cfg.ProjectRoot, output, state, scopeHints)
	if writeErr != nil {
		return writeErr
	}

	intent := &DistillIntent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Files:     targetFiles,
		Phase:     "write",
	}
	if err := WriteIntentFile(cfg.ProjectRoot, intent); err != nil {
		return err
	}

	// Step 10: Phase "commit" — rename .pending → target
	if err := CommitPendingFiles(targetFiles); err != nil {
		return err
	}

	// Step 11: Compute and store metrics, save state
	statePath := filepath.Join(cfg.ProjectRoot, ".ralph", "distill-state.json")
	metrics := ComputeDistillMetrics(string(learningsContent), output)
	state.Metrics = metrics
	if err := SaveDistillState(statePath, state); err != nil {
		return fmt.Errorf("runner: distill: save state: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Distillation metrics: %d→%d entries, %d stale removed, %d [needs-formatting] fixed, %d T1 promotions\n",
		metrics.EntriesBefore, metrics.EntriesAfter, metrics.StaleRemoved, metrics.NeedsFormattingFixed, metrics.T1Promotions)

	// Step 12: Delete intent file (distillation complete)
	if err := DeleteIntentFile(cfg.ProjectRoot); err != nil {
		return err
	}

	// Step 13: Generate index
	if err := WriteDistillIndex(cfg.ProjectRoot); err != nil {
		return err
	}

	return nil
}

// readExistingRules reads all ralph-*.md files and concatenates their content.
func readExistingRules(projectRoot string) string {
	rulesDir := filepath.Join(projectRoot, ".ralph", "rules")
	matches, _ := filepath.Glob(filepath.Join(rulesDir, "ralph-*.md"))

	if len(matches) == 0 {
		return "No existing rule files."
	}

	var sb strings.Builder
	for _, m := range matches {
		name := filepath.Base(m)
		if name == "ralph-index.md" {
			continue
		}
		content, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		sb.WriteString("### " + name + "\n")
		sb.WriteString(string(content))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// containsString checks if a string is in a slice.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
