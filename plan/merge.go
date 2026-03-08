package plan

import (
	"regexp"
	"strings"
)

// storyHeaderRe matches story section headers like "## Story 1.1" or "## Story 12.3".
var storyHeaderRe = regexp.MustCompile(`(?m)^## Story \d+\.\d+`)

// MergeInto merges generated plan content into existing plan content.
// Deduplication: stories present in both are kept from existing (preserving [x] marks).
// New stories from generated are appended at the end.
// Order: all existing content first, then new stories from generated.
// Pure function: no I/O, no side effects.
func MergeInto(existing, generated []byte) ([]byte, error) {
	existingStr := string(existing)
	generatedStr := string(generated)

	if len(existing) == 0 {
		return generated, nil
	}
	if len(generated) == 0 {
		return existing, nil
	}

	// Extract story headers from existing
	existingHeaders := make(map[string]bool)
	for _, match := range storyHeaderRe.FindAllString(existingStr, -1) {
		existingHeaders[match] = true
	}

	// Split generated into story sections
	newSections := splitBySections(generatedStr)

	// Collect only new stories (not in existing)
	var toAppend []string
	for _, section := range newSections {
		header := storyHeaderRe.FindString(section)
		if header == "" {
			continue // skip preamble or non-story content
		}
		if existingHeaders[header] {
			continue // deduplicate
		}
		toAppend = append(toAppend, section)
	}

	if len(toAppend) == 0 {
		return existing, nil
	}

	// Append new stories to existing
	result := strings.TrimRight(existingStr, "\n") + "\n\n" + strings.Join(toAppend, "\n\n") + "\n"
	return []byte(result), nil
}

// splitBySections splits content by "## Story" headers.
// Each returned string starts with the header line.
func splitBySections(content string) []string {
	indices := storyHeaderRe.FindAllStringIndex(content, -1)
	if len(indices) == 0 {
		return nil
	}

	var sections []string
	for i, idx := range indices {
		start := idx[0]
		var end int
		if i+1 < len(indices) {
			end = indices[i+1][0]
		} else {
			end = len(content)
		}
		sections = append(sections, strings.TrimRight(content[start:end], "\n"))
	}
	return sections
}
