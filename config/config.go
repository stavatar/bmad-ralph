package config

import (
	_ "embed" // Required for //go:embed directive
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaultsYAML []byte

// Config holds all ralph configuration parameters.
// Parsed once at startup, passed by pointer, NEVER mutated at runtime.
// Exception: RunID is set once by cmd/ralph before entering the runner (pre-run initialization).
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
	SerenaEnabled   bool `yaml:"serena_enabled"`
	LearningsBudget int  `yaml:"learnings_budget"`
	DistillCooldown int  `yaml:"distill_cooldown"`
	DistillTimeout  int  `yaml:"distill_timeout"`
	StuckThreshold    int     `yaml:"stuck_threshold"`
	SimilarityWindow  int     `yaml:"similarity_window"`
	SimilarityWarn    float64 `yaml:"similarity_warn"`
	SimilarityHard    float64 `yaml:"similarity_hard"`
	BudgetMaxUSD      float64 `yaml:"budget_max_usd"`
	BudgetWarnPct     int     `yaml:"budget_warn_pct"`
	ModelPricing        map[string]Pricing `yaml:"model_pricing"`
	LogDir              string             `yaml:"log_dir"`
	StoriesDir          string             `yaml:"stories_dir"`
	ProjectRoot         string             `yaml:"-"`
	RunID               string             `yaml:"-"` // Runtime-only: UUID v4 set by cmd/ralph, not from config file
}

// CLIFlags holds command-line flag values for the three-level cascade:
// CLI flags > config file > embedded defaults.
// Pointer fields: nil = flag not set (use config/default), non-nil = explicitly set.
type CLIFlags struct {
	MaxTurns            *int
	MaxIterations       *int
	MaxReviewIterations *int
	GatesEnabled        *bool
	GatesCheckpoint     *int
	ReviewEvery         *int
	ModelExecute        *string
	ModelReview         *string
	AlwaysExtract       *bool
}

func defaultConfig() *Config {
	var cfg Config
	// Embedded defaults are compiled into the binary.
	// Parsing failure indicates corrupt defaults.yaml — programming error.
	if err := yaml.Unmarshal(defaultsYAML, &cfg); err != nil {
		panic("config: embedded defaults.yaml: " + err.Error())
	}
	return &cfg
}

// applyCLIFlags applies non-nil CLI flag values to the config, overriding
// any values set by config file or embedded defaults.
func applyCLIFlags(cfg *Config, flags CLIFlags) {
	if flags.MaxTurns != nil {
		cfg.MaxTurns = *flags.MaxTurns
	}
	if flags.MaxIterations != nil {
		cfg.MaxIterations = *flags.MaxIterations
	}
	if flags.MaxReviewIterations != nil {
		cfg.MaxReviewIterations = *flags.MaxReviewIterations
	}
	if flags.GatesEnabled != nil {
		cfg.GatesEnabled = *flags.GatesEnabled
	}
	if flags.GatesCheckpoint != nil {
		cfg.GatesCheckpoint = *flags.GatesCheckpoint
	}
	if flags.ReviewEvery != nil {
		cfg.ReviewEvery = *flags.ReviewEvery
	}
	if flags.ModelExecute != nil {
		cfg.ModelExecute = *flags.ModelExecute
	}
	if flags.ModelReview != nil {
		cfg.ModelReview = *flags.ModelReview
	}
	if flags.AlwaysExtract != nil {
		cfg.AlwaysExtract = *flags.AlwaysExtract
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
			applyCLIFlags(cfg, flags)
			if vErr := cfg.Validate(); vErr != nil {
				return nil, vErr
			}
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
		applyCLIFlags(cfg, flags)
		if vErr := cfg.Validate(); vErr != nil {
			return nil, vErr
		}
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	applyCLIFlags(cfg, flags)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks Config field constraints and returns a descriptive error on first violation.
func (c *Config) Validate() error {
	if c.MaxTurns <= 0 {
		return fmt.Errorf("config: validate: max_turns must be > 0, got %d", c.MaxTurns)
	}
	if c.MaxIterations <= 0 {
		return fmt.Errorf("config: validate: max_iterations must be > 0, got %d", c.MaxIterations)
	}
	if c.MaxReviewIterations <= 0 {
		return fmt.Errorf("config: validate: max_review_iterations must be > 0, got %d", c.MaxReviewIterations)
	}
	switch c.ReviewMinSeverity {
	case "", "HIGH", "MEDIUM", "LOW":
		// valid
	default:
		return fmt.Errorf("config: validate: review_min_severity must be HIGH, MEDIUM, LOW, or empty, got %q", c.ReviewMinSeverity)
	}
	if c.GatesCheckpoint < 0 {
		return fmt.Errorf("config: validate: gates_checkpoint must be >= 0, got %d", c.GatesCheckpoint)
	}
	if c.DistillCooldown < 0 {
		return fmt.Errorf("config: validate: distill_cooldown must be >= 0, got %d", c.DistillCooldown)
	}
	if c.DistillTimeout <= 0 {
		return fmt.Errorf("config: validate: distill_timeout must be > 0, got %d", c.DistillTimeout)
	}
	if c.StuckThreshold < 0 {
		return fmt.Errorf("config: validate: stuck_threshold must be >= 0, got %d", c.StuckThreshold)
	}
	if c.LearningsBudget <= 0 {
		return fmt.Errorf("config: validate: learnings_budget must be > 0, got %d", c.LearningsBudget)
	}
	if c.BudgetMaxUSD < 0 {
		return fmt.Errorf("config: validate: budget_max_usd must be >= 0, got %.2f", c.BudgetMaxUSD)
	}
	if c.BudgetMaxUSD > 0 && (c.BudgetWarnPct < 1 || c.BudgetWarnPct > 99) {
		return fmt.Errorf("config: validate: budget_warn_pct must be 1-99, got %d", c.BudgetWarnPct)
	}
	if c.SimilarityWindow < 0 {
		return fmt.Errorf("config: validate: similarity_window must be >= 0, got %d", c.SimilarityWindow)
	}
	if c.SimilarityWindow > 0 {
		if c.SimilarityWarn <= 0.0 || c.SimilarityWarn >= 1.0 {
			return fmt.Errorf("config: validate: similarity_warn must be in (0.0, 1.0), got %f", c.SimilarityWarn)
		}
		if c.SimilarityHard <= 0.0 || c.SimilarityHard >= 1.0 {
			return fmt.Errorf("config: validate: similarity_hard must be in (0.0, 1.0), got %f", c.SimilarityHard)
		}
		if c.SimilarityWarn >= c.SimilarityHard {
			return fmt.Errorf("config: validate: similarity_warn must be less than similarity_hard")
		}
	}
	return nil
}

// ResolvePath resolves a file through the three-level fallback chain:
//  1. Project-level: <ProjectRoot>/.ralph/<name>
//  2. Global-level: ~/.config/ralph/<name>
//  3. Embedded: provided fallback content
//
// Returns the file content, a source description ("project", "global", or "embedded"),
// and any error. If no level provides content, returns a descriptive error.
func (c *Config) ResolvePath(name string, embedded []byte) ([]byte, string, error) {
	// 1. Project-level override
	projectPath := filepath.Join(c.ProjectRoot, ".ralph", name)
	if data, err := os.ReadFile(projectPath); err == nil {
		return data, "project", nil
	}

	// 2. Global-level override
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".config", "ralph", name)
		if data, err := os.ReadFile(globalPath); err == nil {
			return data, "global", nil
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: config: resolve %q: home dir unavailable: %v\n", name, err)
	}

	// 3. Embedded fallback
	if len(embedded) > 0 {
		return embedded, "embedded", nil
	}

	return nil, "", fmt.Errorf("config: resolve %q: not found in project, global, or embedded", name)
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
