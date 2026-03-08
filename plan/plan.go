package plan

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/plan.md
var planPrompt string

//go:embed prompts/review.md
var reviewPrompt string

// PlanPrompt returns the embedded plan prompt template for testing.
func PlanPrompt() string { return planPrompt }

// ReviewPrompt returns the embedded review prompt template for testing.
func ReviewPrompt() string { return reviewPrompt }

// unreplacedPlaceholderRe matches __UPPER_CASE__ placeholder patterns left after assembly.
var unreplacedPlaceholderRe = regexp.MustCompile(`__[A-Z_0-9]+__`)

// PlanInput represents a single input document for plan generation.
// File and Role are set from config or CLI flags.
// Content is populated by cmd/ralph/plan.go via os.ReadFile (plan/ never reads files).
type PlanInput struct {
	File    string // path to the file
	Role    string // semantic role: "requirements", "technical_context", etc.
	Content []byte // file content — populated by cmd/ layer via os.ReadFile
}

// defaultRoles maps well-known filenames to semantic roles for BMad autodiscovery.
var defaultRoles = map[string]string{
	"prd.md":            "requirements",
	"architecture.md":   "technical_context",
	"ux-design.md":      "design_context",
	"front-end-spec.md": "ui_spec",
}

// ResolveRole returns the role for an input file.
// Priority: explicit role > default mapping by basename > empty string.
// In single-doc mode, returns "" (no roles assigned).
func ResolveRole(filename string, explicitRole string, singleDoc bool) string {
	if singleDoc {
		return ""
	}
	if explicitRole != "" {
		return explicitRole
	}
	base := filepath.Base(filename)
	return defaultRoles[base]
}

// PlanOpts configures a plan generation invocation.
type PlanOpts struct {
	Inputs          []PlanInput
	OutputPath      string // empty → falls back to cfg.PlanOutputPath
	Merge           bool
	NoReview        bool
	MaxRetries      int
	ExistingContent []byte     // content of existing sprint-tasks.md for merge mode; populated by cmd/ralph/plan.go
	CompletedTasks  string     // completed tasks text for replan mode (prepended to output)
	ProgressWriter  io.Writer  // writer for progress output (default: os.Stderr); set in cmd/ layer
}

// templateData holds Stage 1 data for plan prompt template rendering.
type templateData struct {
	Inputs         []PlanInput
	OutputPath     string
	Merge          bool
	Replan         bool
	CompletedTasks string
}

// Run generates sprint-tasks.md from input documents via a Claude session.
// If NoReview is false, a review session follows generation.
// On review OK or NoReview: writes file atomically and returns nil.
// On review ISSUES: retries generation with InjectFeedback up to MaxRetries times.
// After MaxRetries exhausted: accepts the best plan automatically (no gate prompt).
func Run(ctx context.Context, cfg *config.Config, opts PlanOpts) error {
	outputPath := opts.OutputPath
	if outputPath == "" {
		outputPath = cfg.PlanOutputPath
	}

	// Use existing content from opts (populated by cmd/ralph/plan.go, never read here).
	existing := string(opts.ExistingContent)

	// Generate prompt
	prompt, err := GeneratePrompt(opts.Inputs, outputPath, opts.Merge, existing, opts.CompletedTasks)
	if err != nil {
		return fmt.Errorf("plan: generate: %w", err)
	}

	generatedPlan, err := runGenerate(ctx, cfg, prompt, "")
	if err != nil {
		return err
	}

	// Review loop (if enabled): retry up to MaxRetries times on ISSUES, then auto-proceed.
	if !opts.NoReview {
		pw := opts.ProgressWriter
		if pw == nil {
			pw = os.Stderr
		}

		maxRetries := opts.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 1
		}

		for attempt := 0; attempt < maxRetries; attempt++ {
			issues, reviewErr := runReview(ctx, cfg, opts.Inputs, generatedPlan)
			if reviewErr != nil {
				return reviewErr
			}
			if issues == "" {
				break // review passed
			}

			// Retry: regenerate with reviewer feedback
			fmt.Fprintf(pw, "Retry генерации с feedback (попытка %d/%d)...", attempt+1, maxRetries)
			retryStart := time.Now()

			retryPlan, retryErr := runGenerate(ctx, cfg, prompt, issues)
			if retryErr != nil {
				fmt.Fprintln(pw)
				return retryErr
			}
			generatedPlan = retryPlan
			fmt.Fprintf(pw, " завершён (%s)\n", time.Since(retryStart).Round(time.Second))

			if attempt == maxRetries-1 {
				// All retries done — review final result, then auto-proceed regardless
				finalIssues, finalReviewErr := runReview(ctx, cfg, opts.Inputs, generatedPlan)
				if finalReviewErr != nil {
					return finalReviewErr
				}
				if finalIssues != "" {
					fmt.Fprintf(pw, "\nReview выявил проблемы после %d retry, принимаем план автоматически:\n%s\n",
						maxRetries, finalIssues)
				}
			}
		}
	}

	// Prepend completed tasks for replan mode (AC #1: completed [x] preserved at start)
	finalContent := []byte(generatedPlan)
	if opts.CompletedTasks != "" {
		finalContent = append([]byte(opts.CompletedTasks+"\n\n"), finalContent...)
	}

	// Merge if requested
	if opts.Merge && len(opts.ExistingContent) > 0 {
		merged, mergeErr := MergeInto(opts.ExistingContent, finalContent)
		if mergeErr != nil {
			return fmt.Errorf("plan: merge: %w", mergeErr)
		}
		finalContent = merged
	}

	// Write output atomically
	absPath := filepath.Join(cfg.ProjectRoot, outputPath)
	if err := writeAtomic(absPath, finalContent); err != nil {
		return fmt.Errorf("plan: write: %w", err)
	}

	return nil
}

// runGenerate calls Claude for plan generation. If feedback is non-empty,
// it is injected via InjectFeedback for retry.
func runGenerate(ctx context.Context, cfg *config.Config, prompt, feedback string) (string, error) {
	raw, execErr := session.Execute(ctx, session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   cfg.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
		InjectFeedback:             feedback,
	})
	if execErr != nil {
		return "", fmt.Errorf("plan: generate: %w", execErr)
	}

	result, err := session.ParseResult(raw, 0)
	if err != nil {
		return "", fmt.Errorf("plan: generate: parse: %w", err)
	}
	return result.Output, nil
}

// runReview runs a clean review session for the generated plan.
// Returns issues text (non-empty if reviewer found problems) or empty string on OK.
func runReview(ctx context.Context, cfg *config.Config, inputs []PlanInput, generatedPlan string) (string, error) {
	rp, rpErr := GenerateReviewPrompt(inputs, generatedPlan)
	if rpErr != nil {
		return "", fmt.Errorf("plan: review: %w", rpErr)
	}

	reviewRaw, reviewExecErr := session.Execute(ctx, session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Prompt:                     rp,
		MaxTurns:                   cfg.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	})
	if reviewExecErr != nil {
		return "", fmt.Errorf("plan: review: %w", reviewExecErr)
	}

	reviewResult, rpErr := session.ParseResult(reviewRaw, 0)
	if rpErr != nil {
		return "", fmt.Errorf("plan: review: parse: %w", rpErr)
	}

	trimmed := strings.TrimSpace(reviewResult.Output)
	if strings.HasPrefix(trimmed, "OK") {
		return "", nil
	}
	return trimmed, nil
}


// GeneratePrompt assembles the plan prompt from inputs and options.
// Stage 1: text/template renders structure (typed headers, merge conditional).
// Stage 2: strings.Replace injects user content safely via __CONTENT_N__ placeholders.
func GeneratePrompt(inputs []PlanInput, outputPath string, merge bool, existing string, completedTasks string) (string, error) {
	data := templateData{
		Inputs:         inputs,
		OutputPath:     outputPath,
		Merge:          merge,
		Replan:         completedTasks != "",
		CompletedTasks: completedTasks,
	}

	// Build Stage 2 replacements: __CONTENT_N__ for each input
	replacements := make(map[string]string, len(inputs)+2)
	for i, inp := range inputs {
		key := fmt.Sprintf("__CONTENT_%d__", i)
		replacements[key] = string(inp.Content)
	}
	if merge {
		replacements["__EXISTING__"] = existing
	}
	if completedTasks != "" {
		replacements["__COMPLETED_TASKS__"] = completedTasks
	}

	// Stage 1: text/template processing
	tmpl, err := template.New("plan").Option("missingkey=error").Parse(planPrompt)
	if err != nil {
		return "", fmt.Errorf("plan: generate prompt: parse: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("plan: generate prompt: execute: %w", err)
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

	// Post-assembly validation: detect unreplaced __PLACEHOLDER__ patterns.
	if matches := unreplacedPlaceholderRe.FindAllString(result, -1); len(matches) > 0 {
		return "", fmt.Errorf("plan: generate prompt: unreplaced placeholders: %v", matches)
	}

	return result, nil
}

// GenerateReviewPrompt assembles the review prompt from inputs and the generated plan.
// Stage 1: text/template renders structure (input document headers).
// Stage 2: strings.Replace injects content (__CONTENT_N__) and plan (__PLAN__).
func GenerateReviewPrompt(inputs []PlanInput, generatedPlan string) (string, error) {
	data := templateData{
		Inputs: inputs,
	}

	// Build Stage 2 replacements
	replacements := make(map[string]string, len(inputs)+1)
	for i, inp := range inputs {
		key := fmt.Sprintf("__CONTENT_%d__", i)
		replacements[key] = string(inp.Content)
	}
	replacements["__PLAN__"] = generatedPlan

	// Stage 1: text/template processing
	tmpl, err := template.New("review").Option("missingkey=error").Parse(reviewPrompt)
	if err != nil {
		return "", fmt.Errorf("plan: review prompt: parse: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("plan: review prompt: execute: %w", err)
	}

	// Stage 2: deterministic string replacements (sorted by key)
	result := buf.String()
	keys := make([]string, 0, len(replacements))
	for k := range replacements {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = strings.ReplaceAll(result, k, replacements[k])
	}

	// Post-assembly validation
	if matches := unreplacedPlaceholderRe.FindAllString(result, -1); len(matches) > 0 {
		return "", fmt.Errorf("plan: review prompt: unreplaced placeholders: %v", matches)
	}

	return result, nil
}

// writeAtomic writes data to path atomically using temp file + rename.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".ralph-plan-*")
	if err != nil {
		return fmt.Errorf("plan: write atomic: create temp: %w", err)
	}
	defer os.Remove(f.Name()) // cleanup on error
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("plan: write atomic: write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("plan: write atomic: close: %w", err)
	}
	return os.Rename(f.Name(), path)
}
