package runner

import (
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

// DiffStats holds git diff statistics for a task.
type DiffStats struct {
	FilesChanged int      `json:"files_changed"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
	Packages     []string `json:"packages"`
}

// ReviewFinding represents a single code review finding.
type ReviewFinding struct {
	Severity    string `json:"severity"`
	Description string `json:"description"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Agent       string `json:"agent,omitempty"`
}

// AgentFindingStats holds per-agent severity counts aggregated across a run.
type AgentFindingStats struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// LatencyBreakdown tracks time spent in different phases.
type LatencyBreakdown struct {
	SessionMs int64 `json:"session_ms"`
	GitMs     int64 `json:"git_ms"`
	GateMs    int64 `json:"gate_ms"`
	ReviewMs  int64 `json:"review_ms"`
	DistillMs int64 `json:"distill_ms"`
}

// GateStats tracks gate interaction metrics.
type GateStats struct {
	TotalPrompts int    `json:"total_prompts"`
	Approvals    int    `json:"approvals"`
	Rejections   int    `json:"rejections"`
	Skips        int    `json:"skips"`
	TotalWaitMs  int64  `json:"total_wait_ms"`
	LastAction   string `json:"last_action"`
}

// ErrorStats tracks error occurrences during a run.
type ErrorStats struct {
	TotalErrors int      `json:"total_errors"`
	Categories  []string `json:"categories"`
}

// TaskMetrics holds metrics for a single task within a run.
type TaskMetrics struct {
	Name                string            `json:"name"`
	Status              string            `json:"status"`
	CommitSHA           string            `json:"commit_sha"`
	StartTime           time.Time         `json:"start_time"`
	EndTime             time.Time         `json:"end_time"`
	DurationMs          int64             `json:"duration_ms"`
	InputTokens         int               `json:"input_tokens"`
	OutputTokens        int               `json:"output_tokens"`
	CacheTokens         int               `json:"cache_read_tokens"`
	CacheCreationTokens int               `json:"cache_creation_tokens"`
	CostUSD             float64           `json:"cost_usd"`
	NumTurns            int               `json:"num_turns"`
	Sessions            int               `json:"sessions"`
	Diff                *DiffStats        `json:"diff,omitempty"`
	Findings            []ReviewFinding   `json:"findings,omitempty"`
	FindingsProgression []int             `json:"findings_progression,omitempty"`
	CycleDurationsMs    []int64           `json:"cycle_durations_ms,omitempty"`
	Latency             *LatencyBreakdown `json:"latency,omitempty"`
	Gate         *GateStats        `json:"gate,omitempty"`
	Errors       *ErrorStats       `json:"errors,omitempty"`
}

// RunMetrics holds accumulated metrics for an entire run.

// SerenaSyncMetrics holds metrics from a Serena memory sync session.
type SerenaSyncMetrics struct {
	Status     string  `json:"status"`
	DurationMs int64   `json:"duration_ms"`
	TokensIn   int     `json:"tokens_input,omitempty"`
	TokensOut  int     `json:"tokens_output,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
}

type RunMetrics struct {
	RunID               string        `json:"run_id"`
	StartTime           time.Time     `json:"start_time"`
	EndTime             time.Time     `json:"end_time"`
	DurationMs          int64         `json:"duration_ms"`
	Tasks               []TaskMetrics `json:"tasks"`
	InputTokens         int           `json:"input_tokens"`
	OutputTokens        int           `json:"output_tokens"`
	CacheTokens         int           `json:"cache_read_tokens"`
	CacheCreationTokens int           `json:"cache_creation_tokens"`
	CostUSD             float64       `json:"cost_usd"`
	NumTurns       int           `json:"num_turns"`
	TotalSessions  int           `json:"total_sessions"`
	TasksCompleted int           `json:"tasks_completed"`
	TasksFailed    int           `json:"tasks_failed"`
	TasksSkipped   int                          `json:"tasks_skipped"`
	AgentStats     map[string]*AgentFindingStats `json:"agent_stats,omitempty"`
	SerenaSync     *SerenaSyncMetrics            `json:"serena_sync,omitempty"`
}

// taskAccumulator is internal mutable state for the current task.
type taskAccumulator struct {
	name                string
	startTime           time.Time
	inputTokens         int
	outputTokens        int
	cacheTokens         int
	cacheCreationTokens int
	costUSD             float64
	numTurns     int
	sessions     int
	diff                *DiffStats
	findings            []ReviewFinding
	findingsProgression []int
	cycleDurationsMs    []int64
	gate                *GateStats
	errors       *ErrorStats
	latency      *LatencyBreakdown
}

// MetricsCollector accumulates session metrics across a run.
// It is a concrete struct (not interface) — testable via Finish() inspection.
type MetricsCollector struct {
	runID     string
	startTime time.Time
	pricing   map[string]config.Pricing
	tasks     []TaskMetrics
	current   *taskAccumulator

	// Run-level accumulators
	totalInput         int
	totalOutput        int
	totalCache         int
	totalCacheCreation int
	totalCost          float64
	totalTurns    int
	totalSessions int
	agentStats          map[string]*AgentFindingStats
	serenaSync          *SerenaSyncMetrics
	serenaSyncCount     int
	serenaSyncFailCount int
}

// NewMetricsCollector creates a MetricsCollector for the given run.
// pricing maps model IDs to token pricing rates for cost calculation.
func NewMetricsCollector(runID string, pricing map[string]config.Pricing) *MetricsCollector {
	return &MetricsCollector{
		runID:     runID,
		startTime: time.Now(),
		pricing:   pricing,
	}
}

// StartTask begins tracking a new task.
func (mc *MetricsCollector) StartTask(name string) {
	mc.current = &taskAccumulator{
		name:      name,
		startTime: time.Now(),
	}
}

// RecordSession records token usage from a completed session and calculates cost.
// metrics may be nil (graceful degradation when usage data absent).
// When metrics.CostUSD > 0 (from Claude CLI's total_cost_usd), uses it as authoritative cost.
// Falls back to pricing table recalculation when CLI cost is missing or zero.
// model is the Claude model ID used for the session (for pricing lookup on fallback).
// Returns the resolved model used for pricing (may differ from input if fallback applied).
// Returns original model unchanged if no current task (StartTask not called).
// Returns empty string if pricing map is empty (no fallback available).
// stepType and durationMs are accepted for call-site documentation but not stored;
// latency is tracked separately via RecordLatency.
func (mc *MetricsCollector) RecordSession(metrics *session.SessionMetrics, model, stepType string, durationMs int64) string {
	if mc.current == nil {
		return model
	}
	mc.current.sessions++
	resolvedModel := model
	if metrics != nil {
		mc.current.inputTokens += metrics.InputTokens
		mc.current.outputTokens += metrics.OutputTokens
		mc.current.cacheTokens += metrics.CacheReadTokens
		mc.current.cacheCreationTokens += metrics.CacheCreationTokens
		mc.current.numTurns += metrics.NumTurns

		// Prefer CLI-reported total_cost_usd when available; fall back to recalculation.
		if metrics.CostUSD > 0 {
			mc.current.costUSD += metrics.CostUSD
		} else {
			// Cost calculation using pricing table.
			p, ok := mc.pricing[model]
			if !ok {
				resolvedModel = config.MostExpensiveModel(mc.pricing)
				if resolvedModel != "" {
					p = mc.pricing[resolvedModel]
				}
			}
			cost := (float64(metrics.InputTokens)*p.InputPer1M +
				float64(metrics.OutputTokens)*p.OutputPer1M +
				float64(metrics.CacheReadTokens)*p.CachePer1M) / 1_000_000
			mc.current.costUSD += cost
		}
	}
	return resolvedModel
}

// FinishTask finalizes the current task and appends it to the tasks list.
func (mc *MetricsCollector) FinishTask(status, commitSHA string) {
	if mc.current == nil {
		return
	}
	now := time.Now()
	tm := TaskMetrics{
		Name:                mc.current.name,
		Status:              status,
		CommitSHA:           commitSHA,
		StartTime:           mc.current.startTime,
		EndTime:             now,
		DurationMs:          now.Sub(mc.current.startTime).Milliseconds(),
		InputTokens:         mc.current.inputTokens,
		OutputTokens:        mc.current.outputTokens,
		CacheTokens:         mc.current.cacheTokens,
		CacheCreationTokens: mc.current.cacheCreationTokens,
		CostUSD:             mc.current.costUSD,
		NumTurns:            mc.current.numTurns,
		Sessions:            mc.current.sessions,
		Diff:                mc.current.diff,
		Findings:            mc.current.findings,
		FindingsProgression: mc.current.findingsProgression,
		CycleDurationsMs:    mc.current.cycleDurationsMs,
		Gate:                mc.current.gate,
		Errors:              mc.current.errors,
		Latency:             mc.current.latency,
	}
	mc.tasks = append(mc.tasks, tm)

	// Accumulate run totals
	mc.totalInput += mc.current.inputTokens
	mc.totalOutput += mc.current.outputTokens
	mc.totalCache += mc.current.cacheTokens
	mc.totalCacheCreation += mc.current.cacheCreationTokens
	mc.totalCost += mc.current.costUSD
	mc.totalTurns += mc.current.numTurns
	mc.totalSessions += mc.current.sessions

	mc.current = nil
}

// Finish finalizes the run and returns accumulated RunMetrics.
// If a task is still in progress (orphaned by error path), it is auto-finished
// with status "error" to prevent silent data loss.
func (mc *MetricsCollector) Finish() RunMetrics {
	if mc.current != nil {
		mc.FinishTask("error", "")
	}
	now := time.Now()

	var completed, failed, skipped int
	for _, t := range mc.tasks {
		switch t.Status {
		case "completed":
			completed++
		case "failed", "error":
			failed++
		case "skipped":
			skipped++
		}
	}

	return RunMetrics{
		RunID:               mc.runID,
		StartTime:           mc.startTime,
		EndTime:             now,
		DurationMs:          now.Sub(mc.startTime).Milliseconds(),
		Tasks:               mc.tasks,
		InputTokens:         mc.totalInput,
		OutputTokens:        mc.totalOutput,
		CacheTokens:         mc.totalCache,
		CacheCreationTokens: mc.totalCacheCreation,
		CostUSD:             mc.totalCost,
		NumTurns:            mc.totalTurns,
		TotalSessions:       mc.totalSessions,
		TasksCompleted:      completed,
		TasksFailed:         failed,
		TasksSkipped:        skipped,
		AgentStats:          mc.agentStats,
		SerenaSync:          mc.serenaSync,
	}
}

// RecordSerenaSync records Serena memory sync metrics for the run.
// Supports multiple calls (per-task mode): accumulates duration, tokens, and cost.
// Status logic: all success → "success", any fail → "partial", all fail → "failed".
// If result is non-nil and has Metrics, token and cost data are extracted.
// Nil receiver safe: returns immediately when mc == nil.
func (mc *MetricsCollector) RecordSerenaSync(status string, durationMs int64, result *session.SessionResult) {
	if mc == nil {
		return
	}
	mc.serenaSyncCount++
	if status == "failed" || status == "rollback" {
		mc.serenaSyncFailCount++
	}
	if mc.serenaSync == nil {
		mc.serenaSync = &SerenaSyncMetrics{}
	}
	mc.serenaSync.DurationMs += durationMs
	if result != nil && result.Metrics != nil {
		mc.serenaSync.TokensIn += result.Metrics.InputTokens
		mc.serenaSync.TokensOut += result.Metrics.OutputTokens
		mc.serenaSync.CostUSD += result.Metrics.CostUSD
	}
	// Status: all success → "success", any fail → "partial", all fail → "failed"
	if mc.serenaSyncFailCount == 0 {
		mc.serenaSync.Status = "success"
	} else if mc.serenaSyncFailCount == mc.serenaSyncCount {
		mc.serenaSync.Status = "failed"
	} else {
		mc.serenaSync.Status = "partial"
	}
}

// Stub methods — populated by later stories (7.5-7.9).
// RecordGitDiff (7.2) and RecordReview (7.4) are implemented below.

// RecordGitDiff records git diff statistics for the current task.
func (mc *MetricsCollector) RecordGitDiff(stats DiffStats) {
	if mc.current == nil {
		return
	}
	mc.current.diff = &stats
}

// RecordReview records review findings for the current task.
func (mc *MetricsCollector) RecordReview(findings []ReviewFinding) {
	if mc.current == nil {
		return
	}
	mc.current.findings = append(mc.current.findings, findings...)
}

// RecordAgentFinding increments the severity count for the given agent.
// Empty agent is recorded under "unknown". Nil receiver is a no-op.
func (mc *MetricsCollector) RecordAgentFinding(agent, severity string) {
	if mc == nil {
		return
	}
	if agent == "" {
		agent = "unknown"
	}
	if mc.agentStats == nil {
		mc.agentStats = make(map[string]*AgentFindingStats)
	}
	stats := mc.agentStats[agent]
	if stats == nil {
		stats = &AgentFindingStats{}
		mc.agentStats[agent] = stats
	}
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		stats.Critical++
	case "HIGH":
		stats.High++
	case "MEDIUM":
		stats.Medium++
	case "LOW":
		stats.Low++
	}
}

// RecordFindingsCycle appends the findings count for a review cycle to the
// progression slice. Returns true if a hydra pattern is detected (findings
// not decreasing for 2+ consecutive cycles).
func (mc *MetricsCollector) RecordFindingsCycle(count int) bool {
	if mc.current == nil {
		return false
	}
	mc.current.findingsProgression = append(mc.current.findingsProgression, count)
	p := mc.current.findingsProgression
	if len(p) >= 2 && p[len(p)-1] >= p[len(p)-2] {
		return true // hydra: findings not decreasing
	}
	return false
}

// RecordCycleDuration appends the duration of a review cycle (execute + review + git + gate)
// in milliseconds to the current task's cycle durations.
func (mc *MetricsCollector) RecordCycleDuration(ms int64) {
	if mc.current == nil {
		return
	}
	mc.current.cycleDurationsMs = append(mc.current.cycleDurationsMs, ms)
}

// RecordGate records a gate interaction for the current task.
// Merges stats into the task accumulator's gate field: increments counters,
// accumulates TotalWaitMs, and updates LastAction.
func (mc *MetricsCollector) RecordGate(stats GateStats) {
	if mc.current == nil {
		return
	}
	if mc.current.gate == nil {
		mc.current.gate = &GateStats{}
	}
	g := mc.current.gate
	g.TotalPrompts += stats.TotalPrompts
	g.Approvals += stats.Approvals
	g.Rejections += stats.Rejections
	g.Skips += stats.Skips
	g.TotalWaitMs += stats.TotalWaitMs
	g.LastAction = stats.LastAction
}

// RecordRetry records a retry event for the current task (Story 7.5).
func (mc *MetricsCollector) RecordRetry(reason string) {}

// RecordError records an error occurrence for the current task.
// Increments TotalErrors and appends category to Categories slice.
func (mc *MetricsCollector) RecordError(category, message string) {
	if mc.current == nil {
		return
	}
	if mc.current.errors == nil {
		mc.current.errors = &ErrorStats{}
	}
	mc.current.errors.TotalErrors++
	mc.current.errors.Categories = append(mc.current.errors.Categories, category)
}

// RecordLatency merges a latency breakdown into the current task's accumulator.
// Each field is summed (e.g., 3 retries = 3x SessionMs).
func (mc *MetricsCollector) RecordLatency(breakdown LatencyBreakdown) {
	if mc.current == nil {
		return
	}
	if mc.current.latency == nil {
		mc.current.latency = &LatencyBreakdown{}
	}
	mc.current.latency.SessionMs += breakdown.SessionMs
	mc.current.latency.GitMs += breakdown.GitMs
	mc.current.latency.GateMs += breakdown.GateMs
	mc.current.latency.ReviewMs += breakdown.ReviewMs
	mc.current.latency.DistillMs += breakdown.DistillMs
}

// CategorizeError classifies an error as transient, persistent, or unknown
// based on string matching on the error message.
func CategorizeError(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := err.Error()
	for _, pattern := range []string{"407 Proxy", "proxy", "connection refused"} {
		if strings.Contains(msg, pattern) {
			return "proxy"
		}
	}
	for _, pattern := range []string{"rate limit", "timeout", "API error", "connection"} {
		if strings.Contains(msg, pattern) {
			return "transient"
		}
	}
	for _, pattern := range []string{"config", "not found", "permission"} {
		if strings.Contains(msg, pattern) {
			return "persistent"
		}
	}
	return "unknown"
}

// CumulativeCost returns the total cost accumulated so far, including the in-progress task.
func (mc *MetricsCollector) CumulativeCost() float64 {
	if mc.current != nil {
		return mc.totalCost + mc.current.costUSD
	}
	return mc.totalCost
}

// CurrentTaskCost returns the cost accumulated for the in-progress task.
// Returns 0 if no task is in progress.
func (mc *MetricsCollector) CurrentTaskCost() float64 {
	if mc.current != nil {
		return mc.current.costUSD
	}
	return 0
}
