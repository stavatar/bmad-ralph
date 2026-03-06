package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Named constants (L1) for lesson validation thresholds.
const (
	// MaxNewEntriesPerValidation is the max new entries accepted per validation call (G5).
	MaxNewEntriesPerValidation = 5
	// MinEntryContentLength is the minimum content body length in chars (G6).
	MinEntryContentLength = 20
	// SoftDistillationThreshold is the line count triggering NearLimit (BudgetCheck).
	SoftDistillationThreshold = 150
)

// LessonEntry represents a single lesson in LEARNINGS.md.
type LessonEntry struct {
	Category string // e.g. "testing", "errors"
	Topic    string // e.g. "assertion-quality"
	Content  string // body text (may contain VIOLATION: markers)
	Citation string // e.g. "[review, runner/runner_test.go:42]"
}

// LessonsData carries parsed lesson entries and snapshot context for validation.
type LessonsData struct {
	Source      string        // origin identifier (e.g. "execute", "review")
	Entries     []LessonEntry // new entries to validate
	Snapshot    string        // previous LEARNINGS.md content (empty = new file, all entries new)
	BudgetLimit int           // hard budget line limit (from config.LearningsBudget)
}

// BudgetStatus reports LEARNINGS.md line budget state.
type BudgetStatus struct {
	Lines      int  // current line count
	Limit      int  // hard budget limit
	NearLimit  bool // true when Lines >= SoftDistillationThreshold
	OverBudget bool // true when Lines >= Limit (informational only)
}

// headerRegex matches lesson entry headers: ## category: topic [citation]
var headerRegex = regexp.MustCompile(`^##\s+(\w[\w-]*):\s+(.+?)\s+\[(.+)\]$`)

// citationRegex validates citation format: [source, file:line]
var citationRegex = regexp.MustCompile(`\[.*,\s*\S+:\d+\]`)

// needsFormattingTag is the marker added to invalid entries.
const needsFormattingTag = "[needs-formatting]"

// FileKnowledgeWriter validates LEARNINGS.md entries written by Claude.
// Sequential architecture — no concurrent access (L2).
type FileKnowledgeWriter struct {
	projectRoot string
}

// NewFileKnowledgeWriter creates a FileKnowledgeWriter for the given project root.
func NewFileKnowledgeWriter(projectRoot string) *FileKnowledgeWriter {
	return &FileKnowledgeWriter{projectRoot: projectRoot}
}

// WriteProgress writes execution progress. Same behavior as NoOpKnowledgeWriter —
// real progress tracking deferred to future stories.
func (f *FileKnowledgeWriter) WriteProgress(_ context.Context, _ ProgressData) error {
	return nil
}

// Compile-time interface check.
var _ KnowledgeWriter = (*FileKnowledgeWriter)(nil)

// ValidateNewLessons post-validates new lesson entries against 6 quality gates (G1-G6).
// Uses data.Snapshot for snapshot-diff: compares current LEARNINGS.md against snapshot
// to identify new entries, then validates only new entries through quality gates.
// When data.Snapshot is empty (new file), all entries are treated as new.
// Line-count guard: if current < snapshot lines, logs warning and triggers full revalidation.
// Invalid entries are tagged [needs-formatting] in-place — no content is removed (append-only).
// data.BudgetLimit is the hard line limit for G4 (from config.LearningsBudget).
func (f *FileKnowledgeWriter) ValidateNewLessons(_ context.Context, data LessonsData) error {
	learningsPath := f.learningsPath()

	currentContent, err := os.ReadFile(learningsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// NOTE: no RunLogger available in standalone functions — using stdlib log.
			// INFO (not WARN): missing LEARNINGS.md on fresh projects is expected.
			log.Printf("INFO: %s not found, first run — skipping validation", learningsPath)
			return nil
		}
		return fmt.Errorf("runner: validate lessons: %w", err)
	}

	content := string(currentContent)
	snapshot := data.Snapshot
	budgetLimit := data.BudgetLimit
	currentLines := strings.Count(content, "\n")
	snapshotLines := strings.Count(snapshot, "\n")

	// Line-count guard: rewrite detection
	fullRevalidation := false
	if snapshot != "" && currentLines < snapshotLines {
		fmt.Fprintf(os.Stderr, "WARNING: LEARNINGS.md rewrite detected — full revalidation triggered\n")
		fullRevalidation = true
	}

	allEntries := parseEntries(content)
	snapshotEntries := parseEntries(snapshot)

	var newEntries []parsedEntry
	if fullRevalidation || snapshot == "" {
		newEntries = allEntries
	} else {
		newEntries = diffEntries(allEntries, snapshotEntries)
	}

	if len(newEntries) == 0 {
		return nil
	}

	// Semantic dedup (G3): merge entries with same category:topic prefix.
	// Pass allEntries (parsed from current content) so existing entry positions
	// match the content being modified — snapshotEntries have stale line indices.
	content, merged := mergeDedup(content, newEntries, allEntries)

	if merged {
		// Re-parse after merge
		allEntries = parseEntries(content)
		if fullRevalidation || snapshot == "" {
			newEntries = allEntries
		} else {
			newEntries = diffEntries(allEntries, snapshotEntries)
		}
	}

	// Run quality gates (G1, G2, G4, G5, G6)
	lines := strings.Split(content, "\n")
	modified := false
	totalLines := len(lines)

	for i, entry := range newEntries {
		issues := validateEntry(entry, i, totalLines, budgetLimit)
		if len(issues) > 0 {
			tagEntryInContent(&lines, entry, issues)
			modified = true
			fmt.Fprintf(os.Stderr, "WARNING: Entry saved with [needs-formatting] — will be fixed at distillation (entry: %s: %s)\n", entry.Category, entry.Topic)
		}
	}

	if modified || merged {
		var writeContent []byte
		if merged && !modified {
			writeContent = []byte(content)
		} else {
			writeContent = []byte(strings.Join(lines, "\n"))
		}
		if wErr := os.WriteFile(learningsPath, writeContent, 0644); wErr != nil {
			return fmt.Errorf("runner: validate lessons: write: %w", wErr)
		}
	}
	return nil
}

func (f *FileKnowledgeWriter) learningsPath() string {
	return filepath.Join(f.projectRoot, "LEARNINGS.md")
}

// parsedEntry is an internal representation of a parsed lesson entry with position info.
type parsedEntry struct {
	LessonEntry
	rawHeader string // original header line
	startLine int    // 0-based line index of "## ..." header
	endLine   int    // 0-based line index of last content line (exclusive)
}

// parseEntries splits LEARNINGS.md content into parsed entries.
func parseEntries(content string) []parsedEntry {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var entries []parsedEntry
	var current *parsedEntry

	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.endLine = i
				entries = append(entries, *current)
			}
			entry := parseHeader(line)
			current = &parsedEntry{
				LessonEntry: entry,
				rawHeader:   line,
				startLine:   i,
			}
		} else if current != nil {
			// Accumulate content
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, needsFormattingTag) {
				if current.Content != "" {
					current.Content += "\n"
				}
				current.Content += trimmed
			}
		}
	}
	if current != nil {
		current.endLine = len(lines)
		entries = append(entries, *current)
	}

	return entries
}

// parseHeader extracts LessonEntry fields from a header line.
func parseHeader(line string) LessonEntry {
	matches := headerRegex.FindStringSubmatch(line)
	if matches == nil {
		return LessonEntry{Content: line}
	}
	return LessonEntry{
		Category: matches[1],
		Topic:    matches[2],
		Citation: "[" + matches[3] + "]",
	}
}

// validateEntry runs quality gates G1-G6 on an entry. Returns list of failed gate names.
func validateEntry(entry parsedEntry, idx, totalLines, budgetLimit int) []string {
	var issues []string

	// G1: Format check — header must match pattern
	if !headerRegex.MatchString(entry.rawHeader) {
		issues = append(issues, "G1:format")
	}

	// G2: Citation present
	if !citationRegex.MatchString(entry.rawHeader) {
		issues = append(issues, "G2:citation")
	}

	// G4: Budget check — total lines vs hard limit
	if budgetLimit > 0 && totalLines >= budgetLimit {
		issues = append(issues, "G4:budget")
	}

	// G5: Entry cap — max entries per validation call
	if idx >= MaxNewEntriesPerValidation {
		issues = append(issues, "G5:entry-cap")
	}

	// G6: Min content length
	if len(strings.TrimSpace(entry.Content)) < MinEntryContentLength {
		issues = append(issues, "G6:min-content")
	}

	return issues
}

// tagEntryInContent inserts [needs-formatting] tag after the entry header line.
func tagEntryInContent(lines *[]string, entry parsedEntry, issues []string) {
	if entry.startLine >= len(*lines) {
		return
	}
	headerLine := (*lines)[entry.startLine]
	// Don't double-tag
	if strings.Contains(headerLine, needsFormattingTag) {
		return
	}
	(*lines)[entry.startLine] = headerLine + " " + needsFormattingTag
}

// diffEntries returns entries in current that are not in snapshot (by header match).
func diffEntries(current, snapshot []parsedEntry) []parsedEntry {
	snapshotHeaders := make(map[string]bool, len(snapshot))
	for _, e := range snapshot {
		snapshotHeaders[normalizeHeader(e.rawHeader)] = true
	}

	var newEntries []parsedEntry
	for _, e := range current {
		if !snapshotHeaders[normalizeHeader(e.rawHeader)] {
			newEntries = append(newEntries, e)
		}
	}
	return newEntries
}

// normalizeHeader normalizes a header for comparison: lowercase + trim.
func normalizeHeader(header string) string {
	return strings.ToLower(strings.TrimSpace(header))
}

// categoryTopicPrefix extracts normalized "category: topic" from a header for dedup.
func categoryTopicPrefix(header string) string {
	h := normalizeHeader(header)
	// Strip "## " prefix
	h = strings.TrimPrefix(h, "## ")
	// Take everything before " ["
	if idx := strings.Index(h, " ["); idx >= 0 {
		h = h[:idx]
	}
	return strings.TrimSpace(h)
}

// mergeDedup merges new entries with same category:topic prefix into existing entries.
// allEntries must be parsed from the current content (not snapshot) so line positions match.
// Returns modified content and whether any merge occurred.
func mergeDedup(content string, newEntries, allEntries []parsedEntry) (string, bool) {
	merged := false

	// Build set of new entry headers to exclude from "existing" map
	newHeaders := make(map[string]bool, len(newEntries))
	for _, e := range newEntries {
		newHeaders[normalizeHeader(e.rawHeader)] = true
	}

	// Build prefix map from entries NOT in newEntries (i.e., pre-existing entries)
	existingPrefixes := make(map[string]int, len(allEntries)) // prefix → index in allEntries
	for i := range allEntries {
		if newHeaders[normalizeHeader(allEntries[i].rawHeader)] {
			continue
		}
		prefix := categoryTopicPrefix(allEntries[i].rawHeader)
		if prefix != "" {
			existingPrefixes[prefix] = i
		}
	}

	for _, newEntry := range newEntries {
		newPrefix := categoryTopicPrefix(newEntry.rawHeader)
		if newPrefix == "" {
			continue
		}
		if _, found := existingPrefixes[newPrefix]; !found {
			continue
		}
		// Same category:topic — merge content and citations.
		// Re-parse content after each merge to get fresh line positions,
		// avoiding stale indices after insertion/removal shifts.
		freshEntries := parseEntries(content)
		var freshExisting *parsedEntry
		for i := range freshEntries {
			if categoryTopicPrefix(freshEntries[i].rawHeader) == newPrefix &&
				!newHeaders[normalizeHeader(freshEntries[i].rawHeader)] {
				freshExisting = &freshEntries[i]
				break
			}
		}
		if freshExisting == nil {
			continue
		}
		// Re-locate newEntry in freshly parsed content
		var freshNew *parsedEntry
		for i := range freshEntries {
			if normalizeHeader(freshEntries[i].rawHeader) == normalizeHeader(newEntry.rawHeader) {
				freshNew = &freshEntries[i]
				break
			}
		}
		if freshNew == nil {
			continue
		}
		content = mergeEntryContent(content, freshExisting, freshNew)
		merged = true
	}

	return content, merged
}

// mergeEntryContent merges newEntry's content and citation under existing entry,
// then removes the new entry section from content.
func mergeEntryContent(content string, existing, newEntry *parsedEntry) string {
	lines := strings.Split(content, "\n")

	// Append new content under existing entry
	insertIdx := existing.endLine
	if insertIdx > len(lines) {
		insertIdx = len(lines)
	}

	// Merge citation: extract citation from new header and append to existing
	newCitation := ""
	if matches := headerRegex.FindStringSubmatch(newEntry.rawHeader); matches != nil {
		newCitation = matches[3]
	}

	if newCitation != "" && existing.startLine < len(lines) {
		existingHeader := lines[existing.startLine]
		if matches := headerRegex.FindStringSubmatch(existingHeader); matches != nil {
			// Combine citations
			mergedCitation := matches[3] + ", " + newCitation
			lines[existing.startLine] = fmt.Sprintf("## %s: %s [%s]", matches[1], matches[2], mergedCitation)
		}
	}

	// Add new content lines before existing's end
	newContentLines := strings.Split(strings.TrimSpace(newEntry.Content), "\n")
	var contentToInsert []string
	for _, cl := range newContentLines {
		if strings.TrimSpace(cl) != "" {
			contentToInsert = append(contentToInsert, cl)
		}
	}

	if len(contentToInsert) > 0 {
		// Insert content lines at insertIdx
		tail := make([]string, len(lines[insertIdx:]))
		copy(tail, lines[insertIdx:])
		lines = append(lines[:insertIdx], append(contentToInsert, tail...)...)
	}

	// Remove the new entry section (shifted by inserted lines)
	shift := len(contentToInsert)
	removeStart := newEntry.startLine + shift
	removeEnd := newEntry.endLine + shift
	if removeStart < len(lines) {
		if removeEnd > len(lines) {
			removeEnd = len(lines)
		}
		lines = append(lines[:removeStart], lines[removeEnd:]...)
	}

	return strings.Join(lines, "\n")
}

// BudgetCheck reports LEARNINGS.md line budget status.
// Free function (not interface method) to keep KnowledgeWriter 2-method contract.
// Missing file returns zero status with no error.
// Non-NotExist read errors (permission, is-a-directory) return error.
func BudgetCheck(_ context.Context, learningsPath string, limit int) (BudgetStatus, error) {
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return BudgetStatus{Limit: limit}, nil
		}
		return BudgetStatus{}, fmt.Errorf("runner: budget check: %w", err)
	}

	lines := strings.Count(string(content), "\n")
	return BudgetStatus{
		Lines:      lines,
		Limit:      limit,
		NearLimit:  lines >= SoftDistillationThreshold,
		OverBudget: lines >= limit,
	}, nil
}
