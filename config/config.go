package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all ralph configuration parameters.
// Parsed once at startup, passed by pointer, NEVER mutated at runtime.
type Config struct {
	ClaudeCommand       string `yaml:"claude_command"`
	MaxTurns            int    `yaml:"max_turns"`
	MaxIterations       int    `yaml:"max_iterations"`
	MaxReviewIterations int    `yaml:"max_review_iterations"`
	GatesEnabled        bool   `yaml:"gates_enabled"`
	GatesCheckpoint     int    `yaml:"gates_checkpoint"`
	ReviewEvery         int    `yaml:"review_every"`
	ModelExecute        string `yaml:"model_execute"`
	ModelReview         string `yaml:"model_review"`
	ReviewMinSeverity   string `yaml:"review_min_severity"`
	AlwaysExtract       bool   `yaml:"always_extract"`
	SerenaEnabled       bool   `yaml:"serena_enabled"`
	SerenaTimeout       int    `yaml:"serena_timeout"`
	LearningsBudget     int    `yaml:"learnings_budget"`
	LogDir              string `yaml:"log_dir"`
	ProjectRoot         string `yaml:"-"`
}

// CLIFlags holds command-line flag values.
// Pointer fields for "was set" tracking added in Story 1.4.
type CLIFlags struct{}

func defaultConfig() *Config {
	return &Config{
		ClaudeCommand:       "claude",
		MaxTurns:            50,
		MaxIterations:       3,
		MaxReviewIterations: 3,
		GatesEnabled:        false,
		GatesCheckpoint:     0,
		ReviewEvery:         1,
		ModelExecute:        "",
		ModelReview:         "",
		ReviewMinSeverity:   "LOW",
		AlwaysExtract:       false,
		SerenaEnabled:       true,
		SerenaTimeout:       10,
		LearningsBudget:     200,
		LogDir:              ".ralph/logs",
	}
}

// Load reads config from .ralph/config.yaml in the auto-detected project root.
// Missing config file results in all defaults applied (no error).
// Malformed YAML returns a descriptive error.
// Unknown YAML fields are silently ignored.
func Load(flags CLIFlags) (*Config, error) {
	cfg := defaultConfig()

	root, err := detectProjectRoot()
	if err != nil {
		return nil, err
	}
	cfg.ProjectRoot = root

	configPath := filepath.Join(root, ".ralph", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("config: read: %w", err)
	}

	// Guard against yaml.v3 zeroing struct on empty/comment-only documents
	// (GitHub issue #395). Probe with map to detect actual key-value content
	// before unmarshaling into the pre-initialized Config struct.
	var probe map[string]any
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}
	if len(probe) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	return cfg, nil
}

func detectProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("config: getwd: %w", err)
	}
	return detectProjectRootFrom(cwd), nil
}

func detectProjectRootFrom(start string) string {
	// Pass 1: Walk up looking for .ralph/ (highest priority)
	if root := walkUpFor(start, ".ralph"); root != "" {
		return root
	}
	// Pass 2: Walk up looking for .git/ (fallback)
	if root := walkUpFor(start, ".git"); root != "" {
		return root
	}
	// Pass 3: start dir as fallback (warning handled by caller)
	return start
}

func walkUpFor(start, target string) string {
	dir := start
	for {
		candidate := filepath.Join(dir, target)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
