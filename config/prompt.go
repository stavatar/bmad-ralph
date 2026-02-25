package config

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// TemplateData holds structured data for Stage 1 (text/template) processing.
//
// Bool fields are used in Stage 1 as template conditionals:
//
//	{{if .SerenaEnabled}}...{{end}}
//
// String fields exist for type grouping and caller convenience.
// They MUST NOT be referenced via {{.FieldName}} in templates when content
// is user-controlled — user content may contain "{{" which would crash
// text/template. Instead, callers inject string content via Stage 2
// replacements map (e.g., {"__TASK_CONTENT__": actualContent}).
//
// Note: disabled {{if}} blocks leave blank lines in output. Template authors
// should use {{- if .Field -}} trim markers to avoid unwanted whitespace.
type TemplateData struct {
	// Stage 1: bool conditionals for template structure
	SerenaEnabled bool
	GatesEnabled  bool

	// Stage 2: string fields for caller convenience / type grouping.
	// Injected via replacements map, NOT via {{.FieldName}} in templates.
	TaskContent      string
	LearningsContent string
	ClaudeMdContent  string
	FindingsContent  string
}

// AssemblePrompt builds a prompt string using a two-stage assembly process.
//
// Stage 1: text/template processes structural placeholders ({{.Variable}},
// {{if .BoolField}}) using the provided TemplateData. This is safe because
// TemplateData is code-controlled.
//
// Stage 2: strings.Replace injects user-controlled content safely. The template
// engine does NOT re-process Stage 2 output, so user content containing "{{" or
// other template syntax remains literal text.
//
// Replacements are applied in deterministic order (sorted by key) and are flat —
// replacement values MUST NOT contain other replacement placeholders.
//
// missingkey behavior: strict mode (template.Option("missingkey=error")).
// For struct data like TemplateData, unknown fields always cause an execute
// error regardless of this option (Go's struct field resolution). The option
// adds defense-in-depth for potential future use with map-typed data, where
// missing keys would silently output "<no value>" without it.
// This is part of the frozen interface contract.
func AssemblePrompt(tmplContent string, data TemplateData, replacements map[string]string) (string, error) {
	// Stage 1: text/template processing
	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("config: assemble prompt: parse: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("config: assemble prompt: execute: %w", err)
	}

	// Stage 2: deterministic string replacements (sorted by key)
	result := buf.String()
	if len(replacements) > 0 {
		keys := make([]string, 0, len(replacements))
		for k := range replacements {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			result = strings.ReplaceAll(result, k, replacements[k])
		}
	}

	return result, nil
}
