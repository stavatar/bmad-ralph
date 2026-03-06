package runner

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

func TestNewMetricsCollector_SetsRunIDAndStartTime(t *testing.T) {
	mc := NewMetricsCollector("run-123", nil)
	if mc == nil {
		t.Fatal("NewMetricsCollector() returned nil")
	}
	if mc.runID != "run-123" {
		t.Errorf("runID = %q, want %q", mc.runID, "run-123")
	}
	if mc.startTime.IsZero() {
		t.Errorf("startTime is zero, want non-zero")
	}
}

func TestMetricsCollector_FullLifecycle(t *testing.T) {
	mc := NewMetricsCollector("run-abc", nil)

	mc.StartTask("task-1")
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:          1000,
		OutputTokens:         500,
		CacheReadTokens:      200,
		CacheCreationTokens:  150,
		NumTurns:             3,
	}, "", "execute", 5000, 0, 0.0)
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:          800,
		OutputTokens:         400,
		CacheReadTokens:      100,
		CacheCreationTokens:  50,
		NumTurns:             2,
	}, "", "review", 3000, 0, 0.0)
	mc.FinishTask("completed", "abc123")

	mc.StartTask("task-2")
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:          600,
		OutputTokens:         300,
		CacheReadTokens:      50,
		CacheCreationTokens:  30,
		NumTurns:             1,
	}, "", "execute", 2000, 0, 0.0)
	mc.FinishTask("completed", "def456")

	rm := mc.Finish()

	if rm.RunID != "run-abc" {
		t.Errorf("RunID = %q, want %q", rm.RunID, "run-abc")
	}
	if len(rm.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(rm.Tasks))
	}

	// Task 1 assertions
	t1 := rm.Tasks[0]
	if t1.Name != "task-1" {
		t.Errorf("Tasks[0].Name = %q, want %q", t1.Name, "task-1")
	}
	if t1.Status != "completed" {
		t.Errorf("Tasks[0].Status = %q, want %q", t1.Status, "completed")
	}
	if t1.CommitSHA != "abc123" {
		t.Errorf("Tasks[0].CommitSHA = %q, want %q", t1.CommitSHA, "abc123")
	}
	if t1.InputTokens != 1800 {
		t.Errorf("Tasks[0].InputTokens = %d, want 1800", t1.InputTokens)
	}
	if t1.OutputTokens != 900 {
		t.Errorf("Tasks[0].OutputTokens = %d, want 900", t1.OutputTokens)
	}
	if t1.CacheTokens != 300 {
		t.Errorf("Tasks[0].CacheTokens = %d, want 300", t1.CacheTokens)
	}
	if t1.CacheCreationTokens != 200 {
		t.Errorf("Tasks[0].CacheCreationTokens = %d, want 200", t1.CacheCreationTokens)
	}
	if t1.NumTurns != 5 {
		t.Errorf("Tasks[0].NumTurns = %d, want 5", t1.NumTurns)
	}
	if t1.Sessions != 2 {
		t.Errorf("Tasks[0].Sessions = %d, want 2", t1.Sessions)
	}
	if t1.CostUSD != 0.0 {
		t.Errorf("Tasks[0].CostUSD = %f, want 0.0", t1.CostUSD)
	}

	// Task 2 assertions
	t2 := rm.Tasks[1]
	if t2.InputTokens != 600 {
		t.Errorf("Tasks[1].InputTokens = %d, want 600", t2.InputTokens)
	}
	if t2.Sessions != 1 {
		t.Errorf("Tasks[1].Sessions = %d, want 1", t2.Sessions)
	}

	// Run-level totals
	if rm.InputTokens != 2400 {
		t.Errorf("InputTokens = %d, want 2400", rm.InputTokens)
	}
	if rm.OutputTokens != 1200 {
		t.Errorf("OutputTokens = %d, want 1200", rm.OutputTokens)
	}
	if rm.CacheTokens != 350 {
		t.Errorf("CacheTokens = %d, want 350", rm.CacheTokens)
	}
	if rm.CacheCreationTokens != 230 {
		t.Errorf("CacheCreationTokens = %d, want 230", rm.CacheCreationTokens)
	}
	if rm.NumTurns != 6 {
		t.Errorf("NumTurns = %d, want 6", rm.NumTurns)
	}
	if rm.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", rm.TotalSessions)
	}
	if rm.CostUSD != 0.0 {
		t.Errorf("CostUSD = %f, want 0.0", rm.CostUSD)
	}
	if rm.DurationMs < 0 {
		t.Errorf("DurationMs = %d, want >= 0", rm.DurationMs)
	}
	if rm.StartTime.IsZero() {
		t.Errorf("StartTime is zero")
	}
	if rm.EndTime.IsZero() {
		t.Errorf("EndTime is zero")
	}
}

func TestMetricsCollector_NilMetricsInput(t *testing.T) {
	mc := NewMetricsCollector("run-nil", nil)
	mc.StartTask("task-nil")
	mc.RecordSession(nil, "", "execute", 1000, 0, 0.0)
	mc.FinishTask("completed", "sha1")

	rm := mc.Finish()
	if len(rm.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(rm.Tasks))
	}
	if rm.Tasks[0].InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", rm.Tasks[0].InputTokens)
	}
	if rm.Tasks[0].Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", rm.Tasks[0].Sessions)
	}
}

func TestMetricsCollector_NoCurrentTask(t *testing.T) {
	// RecordSession and FinishTask with no StartTask should not panic
	mc := NewMetricsCollector("run-nocurrent", nil)
	mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "", "execute", 500, 0, 0.0)
	mc.FinishTask("completed", "sha1")

	rm := mc.Finish()
	if len(rm.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(rm.Tasks))
	}
}

func TestMetricsCollector_FinishAutoFinishesOrphanedTask(t *testing.T) {
	// Simulates error path: StartTask + RecordSession but NO FinishTask.
	// Finish() must auto-finish with status "error" so partial data is not lost.
	mc := NewMetricsCollector("run-orphan", nil)
	mc.StartTask("orphaned-task")
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:  500,
		OutputTokens: 200,
		NumTurns:     2,
	}, "", "execute", 3000, 0, 0.0)

	rm := mc.Finish()
	if len(rm.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1 (orphaned task auto-finished)", len(rm.Tasks))
	}
	if rm.Tasks[0].Status != "error" {
		t.Errorf("Tasks[0].Status = %q, want %q", rm.Tasks[0].Status, "error")
	}
	if rm.Tasks[0].Name != "orphaned-task" {
		t.Errorf("Tasks[0].Name = %q, want %q", rm.Tasks[0].Name, "orphaned-task")
	}
	if rm.Tasks[0].InputTokens != 500 {
		t.Errorf("Tasks[0].InputTokens = %d, want 500", rm.Tasks[0].InputTokens)
	}
	if rm.Tasks[0].OutputTokens != 200 {
		t.Errorf("Tasks[0].OutputTokens = %d, want 200", rm.Tasks[0].OutputTokens)
	}
	if rm.Tasks[0].Sessions != 1 {
		t.Errorf("Tasks[0].Sessions = %d, want 1", rm.Tasks[0].Sessions)
	}
	// Run-level totals must include orphaned task data
	if rm.InputTokens != 500 {
		t.Errorf("RunMetrics.InputTokens = %d, want 500", rm.InputTokens)
	}
	if rm.TotalSessions != 1 {
		t.Errorf("RunMetrics.TotalSessions = %d, want 1", rm.TotalSessions)
	}
}

func TestMetricsCollector_FinishEmpty(t *testing.T) {
	mc := NewMetricsCollector("run-empty", nil)
	rm := mc.Finish()

	if rm.RunID != "run-empty" {
		t.Errorf("RunID = %q, want %q", rm.RunID, "run-empty")
	}
	if len(rm.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(rm.Tasks))
	}
	if rm.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", rm.InputTokens)
	}
	if rm.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", rm.TotalSessions)
	}
	if rm.TasksCompleted != 0 {
		t.Errorf("TasksCompleted = %d, want 0", rm.TasksCompleted)
	}
	if rm.TasksFailed != 0 {
		t.Errorf("TasksFailed = %d, want 0", rm.TasksFailed)
	}
	if rm.TasksSkipped != 0 {
		t.Errorf("TasksSkipped = %d, want 0", rm.TasksSkipped)
	}
}

// TestMetricsCollector_Finish_AggregateTaskCounts verifies Finish() computes
// TasksCompleted, TasksFailed, TasksSkipped from Tasks[].Status (AC1).
func TestMetricsCollector_Finish_AggregateTaskCounts(t *testing.T) {
	mc := NewMetricsCollector("run-agg", nil)

	mc.StartTask("task-a")
	mc.FinishTask("completed", "abc123")
	mc.StartTask("task-b")
	mc.FinishTask("failed", "")
	mc.StartTask("task-c")
	mc.FinishTask("completed", "def456")
	mc.StartTask("task-d")
	mc.FinishTask("skipped", "")
	mc.StartTask("task-e")
	mc.FinishTask("error", "") // orphaned task auto-finished by Finish()

	rm := mc.Finish()

	if rm.TasksCompleted != 2 {
		t.Errorf("TasksCompleted = %d, want 2", rm.TasksCompleted)
	}
	if rm.TasksFailed != 2 {
		t.Errorf("TasksFailed = %d, want 2 (1 failed + 1 error)", rm.TasksFailed)
	}
	if rm.TasksSkipped != 1 {
		t.Errorf("TasksSkipped = %d, want 1", rm.TasksSkipped)
	}
	if len(rm.Tasks) != 5 {
		t.Errorf("len(Tasks) = %d, want 5", len(rm.Tasks))
	}
}

func TestMetricsCollector_CumulativeCost(t *testing.T) {
	mc := NewMetricsCollector("run-cost", nil)
	// CostUSD is 0 with nil pricing
	if cost := mc.CumulativeCost(); cost != 0.0 {
		t.Errorf("CumulativeCost() = %f, want 0.0", cost)
	}
}

// TestMetricsCollector_CurrentTaskCost_NoTask verifies CurrentTaskCost returns 0 with no task.
func TestMetricsCollector_CurrentTaskCost_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-task-cost", nil)
	if cost := mc.CurrentTaskCost(); cost != 0.0 {
		t.Errorf("CurrentTaskCost() = %f, want 0.0", cost)
	}
}

// TestMetricsCollector_CurrentTaskCost_WithSessions verifies CurrentTaskCost accumulates per-task.
func TestMetricsCollector_CurrentTaskCost_WithSessions(t *testing.T) {
	pricing := map[string]config.Pricing{
		"sonnet": {InputPer1M: 1.0, OutputPer1M: 0, CachePer1M: 0},
	}
	mc := NewMetricsCollector("run-task-cost2", pricing)

	// First task: 2 sessions
	mc.StartTask("task-1")
	mc.RecordSession(&session.SessionMetrics{InputTokens: 1000}, "sonnet", "execute", 1000, 0, 0.0)
	mc.RecordSession(&session.SessionMetrics{InputTokens: 2000}, "sonnet", "review", 2000, 0, 0.0)
	// task-1 cost = (1000+2000) * 1.0 / 1_000_000 = 0.003
	taskCost := mc.CurrentTaskCost()
	wantCost := 0.003
	if taskCost != wantCost {
		t.Errorf("CurrentTaskCost() = %f, want %f", taskCost, wantCost)
	}

	// Finish task-1 and start task-2: cost resets
	mc.FinishTask("completed", "abc")
	mc.StartTask("task-2")
	mc.RecordSession(&session.SessionMetrics{InputTokens: 500}, "sonnet", "execute", 500, 0, 0.0)
	// task-2 cost = 500 * 1.0 / 1_000_000 = 0.0005
	taskCost2 := mc.CurrentTaskCost()
	wantCost2 := 0.0005
	if taskCost2 != wantCost2 {
		t.Errorf("CurrentTaskCost() after new task = %f, want %f", taskCost2, wantCost2)
	}
}

// TestMetricsCollector_CostCalculation verifies cost is calculated from pricing table.
func TestMetricsCollector_CostCalculation(t *testing.T) {
	pricing := map[string]config.Pricing{
		"claude-sonnet-4-20250514": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	}
	mc := NewMetricsCollector("run-cost-calc", pricing)
	mc.StartTask("task-cost")
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 200,
		NumTurns:        1,
	}, "claude-sonnet-4-20250514", "execute", 5000, 0, 0.0)
	mc.FinishTask("completed", "sha1")

	rm := mc.Finish()
	// Expected: (1000*3.0 + 500*15.0 + 200*0.30) / 1_000_000 = (3000 + 7500 + 60) / 1_000_000 = 0.01056
	wantCost := 0.01056
	if rm.Tasks[0].CostUSD != wantCost {
		t.Errorf("Tasks[0].CostUSD = %f, want %f", rm.Tasks[0].CostUSD, wantCost)
	}
	if rm.CostUSD != wantCost {
		t.Errorf("RunMetrics.CostUSD = %f, want %f", rm.CostUSD, wantCost)
	}
}

// TestMetricsCollector_MultiSessionCostAggregation verifies cost aggregates across sessions per task.
func TestMetricsCollector_MultiSessionCostAggregation(t *testing.T) {
	pricing := map[string]config.Pricing{
		"sonnet": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
		"opus":   {InputPer1M: 15.0, OutputPer1M: 75.0, CachePer1M: 1.50},
	}
	mc := NewMetricsCollector("run-multi-cost", pricing)
	mc.StartTask("task-multi")

	// Session 1: sonnet
	mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, "sonnet", "execute", 1000, 0, 0.0)
	// Expected: (1000*3 + 500*15 + 200*0.3) / 1M = 10560 / 1M = 0.01056

	// Session 2: opus
	mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, "opus", "review", 2000, 0, 0.0)
	// Expected: (1000*15 + 500*75 + 200*1.5) / 1M = 52800 / 1M = 0.0528

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	wantCost := 0.01056 + 0.0528 // 0.06336
	if rm.Tasks[0].CostUSD != wantCost {
		t.Errorf("Tasks[0].CostUSD = %f, want %f", rm.Tasks[0].CostUSD, wantCost)
	}
}

// TestMetricsCollector_CumulativeCostIncludesCurrent verifies CumulativeCost includes in-progress task.
func TestMetricsCollector_CumulativeCostIncludesCurrent(t *testing.T) {
	pricing := map[string]config.Pricing{
		"sonnet": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	}
	mc := NewMetricsCollector("run-cumul", pricing)

	// Finish first task
	mc.StartTask("task-1")
	mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, "sonnet", "execute", 1000, 0, 0.0)
	mc.FinishTask("completed", "sha1")
	task1Cost := 0.01056

	// Start second task (in-progress)
	mc.StartTask("task-2")
	mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, "sonnet", "execute", 2000, 0, 0.0)

	// CumulativeCost should include both finished and in-progress
	wantCumul := task1Cost * 2
	got := mc.CumulativeCost()
	if got != wantCumul {
		t.Errorf("CumulativeCost() = %f, want %f", got, wantCumul)
	}
}

// TestMetricsCollector_CostPrecision verifies 1000 sessions * $0.003 = $3.00 (AC7).
func TestMetricsCollector_CostPrecision(t *testing.T) {
	// Design: 1 input token at $3.0/1M = $0.000003
	// 1000 tokens * 1000 sessions = should be $3.0 / 1M * 1000 * 1000 = $3.00
	pricing := map[string]config.Pricing{
		"model": {InputPer1M: 3.0, OutputPer1M: 0, CachePer1M: 0},
	}
	mc := NewMetricsCollector("run-precision", pricing)
	mc.StartTask("precision-task")
	for i := 0; i < 1000; i++ {
		mc.RecordSession(&session.SessionMetrics{
			InputTokens: 1000,
		}, "model", "execute", 100, 0, 0.0)
	}
	mc.FinishTask("completed", "sha1")

	rm := mc.Finish()
	// Expected: 1000 sessions * (1000 * 3.0 / 1_000_000) = 1000 * 0.003 = 3.0
	// AC7: no precision loss visible at 2 decimal places
	if math.Abs(rm.Tasks[0].CostUSD-3.0) > 0.005 {
		t.Errorf("CostUSD = %.6f, want ~3.00 (precision test)", rm.Tasks[0].CostUSD)
	}
}

// TestMetricsCollector_UnknownModelFallback verifies fallback to most expensive model (AC3).
func TestMetricsCollector_UnknownModelFallback(t *testing.T) {
	pricing := map[string]config.Pricing{
		"cheap":  {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
		"pricey": {InputPer1M: 15.0, OutputPer1M: 75.0, CachePer1M: 1.50},
	}
	mc := NewMetricsCollector("run-unknown", pricing)
	mc.StartTask("task-unknown")
	resolved := mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, "unknown-model", "execute", 1000, 0, 0.0)

	if resolved != "pricey" {
		t.Errorf("resolved model = %q, want %q", resolved, "pricey")
	}

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	// Cost should use pricey pricing: (1000*15 + 500*75 + 200*1.5) / 1M = 0.0528
	wantCost := 0.0528
	if rm.Tasks[0].CostUSD != wantCost {
		t.Errorf("CostUSD = %f, want %f", rm.Tasks[0].CostUSD, wantCost)
	}
}

// TestMetricsCollector_CLICostPreferred verifies CLI-reported total_cost_usd is used over recalculation.
func TestMetricsCollector_CLICostPreferred(t *testing.T) {
	pricing := map[string]config.Pricing{
		"claude-sonnet-4-20250514": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	}
	mc := NewMetricsCollector("run-cli-cost", pricing)
	mc.StartTask("task-cli-cost")

	// Session with CLI-reported cost — should use 0.042, NOT recalculate
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 200,
		CostUSD:         0.042,
		NumTurns:        3,
	}, "claude-sonnet-4-20250514", "execute", 5000, 0, 0.0)

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	if rm.Tasks[0].CostUSD != 0.042 {
		t.Errorf("Tasks[0].CostUSD = %f, want 0.042 (CLI cost preferred)", rm.Tasks[0].CostUSD)
	}
	if rm.CostUSD != 0.042 {
		t.Errorf("RunMetrics.CostUSD = %f, want 0.042", rm.CostUSD)
	}
}

// TestMetricsCollector_CLICostZeroFallsBackToRecalculation verifies fallback when CLI cost is zero.
func TestMetricsCollector_CLICostZeroFallsBackToRecalculation(t *testing.T) {
	pricing := map[string]config.Pricing{
		"claude-sonnet-4-20250514": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	}
	mc := NewMetricsCollector("run-fallback", pricing)
	mc.StartTask("task-fallback")

	// Session with zero CLI cost — should recalculate
	mc.RecordSession(&session.SessionMetrics{
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 200,
		CostUSD:         0.0,
		NumTurns:        1,
	}, "claude-sonnet-4-20250514", "execute", 5000, 0, 0.0)

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	// Recalculated: (1000*3.0 + 500*15.0 + 200*0.30) / 1_000_000 = 0.01056
	wantCost := 0.01056
	if rm.Tasks[0].CostUSD != wantCost {
		t.Errorf("Tasks[0].CostUSD = %f, want %f (recalculated fallback)", rm.Tasks[0].CostUSD, wantCost)
	}
}

// TestMetricsCollector_CLICostMixedSessions verifies mixed CLI and recalculated costs aggregate correctly.
func TestMetricsCollector_CLICostMixedSessions(t *testing.T) {
	pricing := map[string]config.Pricing{
		"sonnet": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	}
	mc := NewMetricsCollector("run-mixed", pricing)
	mc.StartTask("task-mixed")

	// Session 1: CLI cost provided
	mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
		CostUSD: 0.05,
	}, "sonnet", "execute", 1000, 0, 0.0)

	// Session 2: no CLI cost, recalculate
	mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, "sonnet", "review", 2000, 0, 0.0)

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	// Expected: 0.05 (CLI) + 0.01056 (recalculated) = 0.06056
	wantCost := 0.05 + 0.01056
	if rm.Tasks[0].CostUSD != wantCost {
		t.Errorf("Tasks[0].CostUSD = %f, want %f (mixed CLI + recalculated)", rm.Tasks[0].CostUSD, wantCost)
	}
}

// TestMetricsCollector_EmptyPricingNoCost verifies zero cost when pricing is nil.
func TestMetricsCollector_EmptyPricingNoCost(t *testing.T) {
	mc := NewMetricsCollector("run-no-pricing", nil)
	mc.StartTask("task-no-pricing")
	resolved := mc.RecordSession(&session.SessionMetrics{
		InputTokens: 1000, OutputTokens: 500,
	}, "any-model", "execute", 1000, 0, 0.0)

	// With nil pricing, MostExpensiveModel returns "" — no fallback available
	if resolved != "" {
		t.Errorf("resolved model = %q, want empty", resolved)
	}

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	if rm.Tasks[0].CostUSD != 0.0 {
		t.Errorf("CostUSD = %f, want 0.0", rm.Tasks[0].CostUSD)
	}
}

func TestRunMetrics_ZeroValue(t *testing.T) {
	var rm RunMetrics
	if rm.RunID != "" {
		t.Errorf("zero RunID = %q, want empty", rm.RunID)
	}
	if rm.Tasks != nil {
		t.Errorf("zero Tasks = %v, want nil", rm.Tasks)
	}
	if rm.InputTokens != 0 {
		t.Errorf("zero InputTokens = %d, want 0", rm.InputTokens)
	}
	if rm.DurationMs != 0 {
		t.Errorf("zero DurationMs = %d, want 0", rm.DurationMs)
	}
}

func TestTaskMetrics_ZeroValue(t *testing.T) {
	var tm TaskMetrics
	if tm.Name != "" {
		t.Errorf("zero Name = %q, want empty", tm.Name)
	}
	if tm.Status != "" {
		t.Errorf("zero Status = %q, want empty", tm.Status)
	}
	if tm.InputTokens != 0 {
		t.Errorf("zero InputTokens = %d, want 0", tm.InputTokens)
	}
	if tm.Diff != nil {
		t.Errorf("zero Diff = %+v, want nil", tm.Diff)
	}
}

// TestMetricsCollector_RecordGitDiff verifies diff stats are stored in TaskMetrics after FinishTask.
func TestMetricsCollector_RecordGitDiff(t *testing.T) {
	mc := NewMetricsCollector("run-diff", nil)
	mc.StartTask("task-with-diff")
	mc.RecordGitDiff(DiffStats{
		FilesChanged: 3,
		Insertions:   42,
		Deletions:    7,
		Packages:     []string{".", "pkg"},
	})
	mc.FinishTask("completed", "abc123")
	rm := mc.Finish()

	if len(rm.Tasks) != 1 {
		t.Fatalf("Tasks len = %d, want 1", len(rm.Tasks))
	}
	tm := rm.Tasks[0]
	if tm.Diff == nil {
		t.Fatal("Diff is nil, want non-nil")
	}
	if tm.Diff.FilesChanged != 3 {
		t.Errorf("Diff.FilesChanged = %d, want 3", tm.Diff.FilesChanged)
	}
	if tm.Diff.Insertions != 42 {
		t.Errorf("Diff.Insertions = %d, want 42", tm.Diff.Insertions)
	}
	if tm.Diff.Deletions != 7 {
		t.Errorf("Diff.Deletions = %d, want 7", tm.Diff.Deletions)
	}
	if len(tm.Diff.Packages) != 2 {
		t.Fatalf("Diff.Packages len = %d, want 2", len(tm.Diff.Packages))
	}
	if tm.Diff.Packages[0] != "." || tm.Diff.Packages[1] != "pkg" {
		t.Errorf("Diff.Packages = %v, want [. pkg]", tm.Diff.Packages)
	}
}

// TestMetricsCollector_RecordGitDiff_NoTask verifies RecordGitDiff is safe without StartTask.
func TestMetricsCollector_RecordGitDiff_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-no-task", nil)
	// Should not panic
	mc.RecordGitDiff(DiffStats{FilesChanged: 1})
}

// TestMetricsCollector_RecordReview verifies findings are stored in TaskMetrics after FinishTask.
func TestMetricsCollector_RecordReview(t *testing.T) {
	mc := NewMetricsCollector("run-review", nil)
	mc.StartTask("task-with-findings")

	findings := []ReviewFinding{
		{Severity: "HIGH", Description: "Missing error assertion"},
		{Severity: "MEDIUM", Description: "Stale doc comment"},
		{Severity: "LOW", Description: "Unused fixture"},
	}
	mc.RecordReview(findings)
	mc.FinishTask("completed", "sha-review")

	rm := mc.Finish()
	if len(rm.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(rm.Tasks))
	}
	if len(rm.Tasks[0].Findings) != 3 {
		t.Fatalf("len(Findings) = %d, want 3", len(rm.Tasks[0].Findings))
	}
	if rm.Tasks[0].Findings[0].Severity != "HIGH" {
		t.Errorf("Findings[0].Severity = %q, want %q", rm.Tasks[0].Findings[0].Severity, "HIGH")
	}
	if rm.Tasks[0].Findings[0].Description != "Missing error assertion" {
		t.Errorf("Findings[0].Description = %q, want %q", rm.Tasks[0].Findings[0].Description, "Missing error assertion")
	}
	if rm.Tasks[0].Findings[1].Severity != "MEDIUM" {
		t.Errorf("Findings[1].Severity = %q, want %q", rm.Tasks[0].Findings[1].Severity, "MEDIUM")
	}
	if rm.Tasks[0].Findings[1].Description != "Stale doc comment" {
		t.Errorf("Findings[1].Description = %q, want %q", rm.Tasks[0].Findings[1].Description, "Stale doc comment")
	}
	if rm.Tasks[0].Findings[2].Severity != "LOW" {
		t.Errorf("Findings[2].Severity = %q, want %q", rm.Tasks[0].Findings[2].Severity, "LOW")
	}
}

// TestMetricsCollector_RecordReview_NoTask verifies RecordReview does not panic without StartTask.
func TestMetricsCollector_RecordReview_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-no-task", nil)
	mc.RecordReview([]ReviewFinding{{Severity: "HIGH", Description: "test"}})
	rm := mc.Finish()
	if len(rm.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(rm.Tasks))
	}
}

// TestMetricsCollector_RecordGate verifies GateStats counters are correctly set for each action type.
func TestMetricsCollector_RecordGate(t *testing.T) {
	mc := NewMetricsCollector("run-gate", nil)
	mc.StartTask("task-gate")

	// Approve action
	mc.RecordGate(GateStats{TotalPrompts: 1, Approvals: 1, TotalWaitMs: 500, LastAction: "approve"})

	mc.FinishTask("completed", "sha-gate")
	rm := mc.Finish()

	if len(rm.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(rm.Tasks))
	}
	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil, want non-nil")
	}
	if g.TotalPrompts != 1 {
		t.Errorf("TotalPrompts = %d, want 1", g.TotalPrompts)
	}
	if g.Approvals != 1 {
		t.Errorf("Approvals = %d, want 1", g.Approvals)
	}
	if g.Rejections != 0 {
		t.Errorf("Rejections = %d, want 0", g.Rejections)
	}
	if g.Skips != 0 {
		t.Errorf("Skips = %d, want 0", g.Skips)
	}
	if g.TotalWaitMs != 500 {
		t.Errorf("TotalWaitMs = %d, want 500", g.TotalWaitMs)
	}
	if g.LastAction != "approve" {
		t.Errorf("LastAction = %q, want %q", g.LastAction, "approve")
	}
}

// TestMetricsCollector_RecordGate_MultipleCallsAccumulate verifies counters accumulate across calls.
func TestMetricsCollector_RecordGate_MultipleCallsAccumulate(t *testing.T) {
	mc := NewMetricsCollector("run-gate-multi", nil)
	mc.StartTask("task-gate-multi")

	mc.RecordGate(GateStats{TotalPrompts: 1, Approvals: 1, TotalWaitMs: 500, LastAction: "approve"})
	mc.RecordGate(GateStats{TotalPrompts: 1, Rejections: 1, TotalWaitMs: 300, LastAction: "quit"})
	mc.RecordGate(GateStats{TotalPrompts: 1, Skips: 1, TotalWaitMs: 200, LastAction: "skip"})

	mc.FinishTask("completed", "sha-multi")
	rm := mc.Finish()

	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil, want non-nil")
	}
	if g.TotalPrompts != 3 {
		t.Errorf("TotalPrompts = %d, want 3", g.TotalPrompts)
	}
	if g.Approvals != 1 {
		t.Errorf("Approvals = %d, want 1", g.Approvals)
	}
	if g.Rejections != 1 {
		t.Errorf("Rejections = %d, want 1", g.Rejections)
	}
	if g.Skips != 1 {
		t.Errorf("Skips = %d, want 1", g.Skips)
	}
	if g.TotalWaitMs != 1000 {
		t.Errorf("TotalWaitMs = %d, want 1000", g.TotalWaitMs)
	}
	if g.LastAction != "skip" {
		t.Errorf("LastAction = %q, want %q (last call wins)", g.LastAction, "skip")
	}
}

// TestMetricsCollector_RecordGate_NoTask verifies RecordGate is safe without StartTask.
func TestMetricsCollector_RecordGate_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-no-task", nil)
	// Should not panic
	mc.RecordGate(GateStats{TotalPrompts: 1, Approvals: 1, TotalWaitMs: 100, LastAction: "approve"})
	rm := mc.Finish()
	if len(rm.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(rm.Tasks))
	}
}

// TestMetricsCollector_RecordGate_RetryOnlyIncrementsPrompts verifies retry does not increment action counters.
func TestMetricsCollector_RecordGate_RetryOnlyIncrementsPrompts(t *testing.T) {
	mc := NewMetricsCollector("run-retry", nil)
	mc.StartTask("task-retry")

	// Retry: TotalPrompts=1 but no action counters set
	mc.RecordGate(GateStats{TotalPrompts: 1, TotalWaitMs: 150, LastAction: "retry"})

	mc.FinishTask("completed", "sha-retry")
	rm := mc.Finish()

	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil, want non-nil")
	}
	if g.TotalPrompts != 1 {
		t.Errorf("TotalPrompts = %d, want 1", g.TotalPrompts)
	}
	if g.Approvals != 0 {
		t.Errorf("Approvals = %d, want 0 (retry should not increment)", g.Approvals)
	}
	if g.Rejections != 0 {
		t.Errorf("Rejections = %d, want 0 (retry should not increment)", g.Rejections)
	}
	if g.Skips != 0 {
		t.Errorf("Skips = %d, want 0 (retry should not increment)", g.Skips)
	}
	if g.TotalWaitMs != 150 {
		t.Errorf("TotalWaitMs = %d, want 150", g.TotalWaitMs)
	}
	if g.LastAction != "retry" {
		t.Errorf("LastAction = %q, want %q", g.LastAction, "retry")
	}
}

// TestMetricsCollector_RecordReview_MultipleReviewCycles verifies findings accumulate across review cycles.
func TestMetricsCollector_RecordReview_MultipleReviewCycles(t *testing.T) {
	mc := NewMetricsCollector("run-multi", nil)
	mc.StartTask("task-multi")

	mc.RecordReview([]ReviewFinding{{Severity: "HIGH", Description: "first"}})
	mc.RecordReview([]ReviewFinding{{Severity: "LOW", Description: "second"}, {Severity: "MEDIUM", Description: "third"}})
	mc.FinishTask("completed", "sha-multi")

	rm := mc.Finish()
	if len(rm.Tasks[0].Findings) != 3 {
		t.Fatalf("len(Findings) = %d, want 3", len(rm.Tasks[0].Findings))
	}
	if rm.Tasks[0].Findings[0].Description != "first" {
		t.Errorf("Findings[0].Description = %q, want %q", rm.Tasks[0].Findings[0].Description, "first")
	}
	if rm.Tasks[0].Findings[1].Description != "second" {
		t.Errorf("Findings[1].Description = %q, want %q", rm.Tasks[0].Findings[1].Description, "second")
	}
	if rm.Tasks[0].Findings[1].Severity != "LOW" {
		t.Errorf("Findings[1].Severity = %q, want %q", rm.Tasks[0].Findings[1].Severity, "LOW")
	}
	if rm.Tasks[0].Findings[2].Description != "third" {
		t.Errorf("Findings[2].Description = %q, want %q", rm.Tasks[0].Findings[2].Description, "third")
	}
}

// --- Story 7.9: CategorizeError ---

func TestCategorizeError_Patterns(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCat  string
	}{
		// Proxy patterns (checked first — takes priority over transient)
		{name: "407 Proxy", err: errors.New("407 Proxy Authentication Required"), wantCat: "proxy"},
		{name: "proxy keyword", err: errors.New("proxy connection failed"), wantCat: "proxy"},
		{name: "connection refused", err: errors.New("connection refused"), wantCat: "proxy"},
		// Transient patterns
		{name: "rate limit", err: errors.New("rate limit exceeded"), wantCat: "transient"},
		{name: "timeout", err: errors.New("request timeout"), wantCat: "transient"},
		{name: "API error", err: errors.New("API error 500"), wantCat: "transient"},
		{name: "connection reset", err: errors.New("connection reset by peer"), wantCat: "transient"},
		// Persistent patterns
		{name: "config", err: errors.New("config file missing"), wantCat: "persistent"},
		{name: "not found", err: errors.New("file not found"), wantCat: "persistent"},
		{name: "permission", err: errors.New("permission denied"), wantCat: "persistent"},
		// Unknown
		{name: "unknown error", err: errors.New("something went wrong"), wantCat: "unknown"},
		{name: "nil error", err: nil, wantCat: "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CategorizeError(tc.err)
			if got != tc.wantCat {
				t.Errorf("CategorizeError(%v) = %q, want %q", tc.err, got, tc.wantCat)
			}
		})
	}
}

// --- Story 7.9: RecordError ---

func TestMetricsCollector_RecordError(t *testing.T) {
	mc := NewMetricsCollector("run-err", nil)
	mc.StartTask("task-err")

	mc.RecordError("transient", "rate limit exceeded")
	mc.RecordError("persistent", "config file missing")

	mc.FinishTask("error", "")
	rm := mc.Finish()

	if rm.Tasks[0].Errors == nil {
		t.Fatal("Errors is nil, want non-nil")
	}
	if rm.Tasks[0].Errors.TotalErrors != 2 {
		t.Errorf("TotalErrors = %d, want 2", rm.Tasks[0].Errors.TotalErrors)
	}
	if len(rm.Tasks[0].Errors.Categories) != 2 {
		t.Fatalf("len(Categories) = %d, want 2", len(rm.Tasks[0].Errors.Categories))
	}
	if rm.Tasks[0].Errors.Categories[0] != "transient" {
		t.Errorf("Categories[0] = %q, want %q", rm.Tasks[0].Errors.Categories[0], "transient")
	}
	if rm.Tasks[0].Errors.Categories[1] != "persistent" {
		t.Errorf("Categories[1] = %q, want %q", rm.Tasks[0].Errors.Categories[1], "persistent")
	}
}

func TestMetricsCollector_RecordError_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-noerr", nil)
	// No StartTask — should not panic
	mc.RecordError("transient", "should be ignored")
}

// --- Story 7.9: RecordLatency ---

func TestMetricsCollector_RecordLatency(t *testing.T) {
	mc := NewMetricsCollector("run-lat", nil)
	mc.StartTask("task-lat")

	mc.RecordLatency(LatencyBreakdown{
		SessionMs: 100,
		GitMs:     20,
		GateMs:    50,
		ReviewMs:  30,
		DistillMs: 10,
	})

	mc.FinishTask("completed", "sha-lat")
	rm := mc.Finish()

	lat := rm.Tasks[0].Latency
	if lat == nil {
		t.Fatal("Latency is nil, want non-nil")
	}
	if lat.SessionMs != 100 {
		t.Errorf("SessionMs = %d, want 100", lat.SessionMs)
	}
	if lat.GitMs != 20 {
		t.Errorf("GitMs = %d, want 20", lat.GitMs)
	}
	if lat.GateMs != 50 {
		t.Errorf("GateMs = %d, want 50", lat.GateMs)
	}
	if lat.ReviewMs != 30 {
		t.Errorf("ReviewMs = %d, want 30", lat.ReviewMs)
	}
	if lat.DistillMs != 10 {
		t.Errorf("DistillMs = %d, want 10", lat.DistillMs)
	}
}

func TestMetricsCollector_RecordLatency_MultipleCallsAccumulate(t *testing.T) {
	mc := NewMetricsCollector("run-lat2", nil)
	mc.StartTask("task-lat2")

	mc.RecordLatency(LatencyBreakdown{SessionMs: 100, GitMs: 10})
	mc.RecordLatency(LatencyBreakdown{SessionMs: 200, GitMs: 20, ReviewMs: 50})

	mc.FinishTask("completed", "sha-lat2")
	rm := mc.Finish()

	lat := rm.Tasks[0].Latency
	if lat == nil {
		t.Fatal("Latency is nil, want non-nil")
	}
	if lat.SessionMs != 300 {
		t.Errorf("SessionMs = %d, want 300", lat.SessionMs)
	}
	if lat.GitMs != 30 {
		t.Errorf("GitMs = %d, want 30", lat.GitMs)
	}
	if lat.ReviewMs != 50 {
		t.Errorf("ReviewMs = %d, want 50", lat.ReviewMs)
	}
	// Symmetric zero assertions for untouched fields
	if lat.GateMs != 0 {
		t.Errorf("GateMs = %d, want 0", lat.GateMs)
	}
	if lat.DistillMs != 0 {
		t.Errorf("DistillMs = %d, want 0", lat.DistillMs)
	}
}

func TestMetricsCollector_RecordLatency_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-nolat", nil)
	// No StartTask — should not panic
	mc.RecordLatency(LatencyBreakdown{SessionMs: 100})
}

// --- Story 8.5: SerenaSyncMetrics tests ---

func TestSerenaSyncMetrics_JSONSerialization(t *testing.T) {
	sm := SerenaSyncMetrics{
		Status:     "success",
		DurationMs: 12000,
		TokensIn:   500,
		TokensOut:  200,
		CostUSD:    0.05,
	}
	data, err := json.Marshal(sm)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s := string(data)

	// Verify field names (AC #1)
	for _, field := range []string{`"status"`, `"duration_ms"`, `"tokens_input"`, `"tokens_output"`, `"cost_usd"`} {
		if !strings.Contains(s, field) {
			t.Errorf("JSON missing field %s: %s", field, s)
		}
	}

	// Verify omitempty: zero tokens/cost should be omitted
	smZero := SerenaSyncMetrics{Status: "skipped", DurationMs: 1000}
	dataZero, err := json.Marshal(smZero)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	sZero := string(dataZero)
	for _, field := range []string{`"tokens_input"`, `"tokens_output"`, `"cost_usd"`} {
		if strings.Contains(sZero, field) {
			t.Errorf("JSON should omit zero field %s: %s", field, sZero)
		}
	}
	// status and duration_ms always present
	for _, field := range []string{`"status"`, `"duration_ms"`} {
		if !strings.Contains(sZero, field) {
			t.Errorf("JSON missing required field %s: %s", field, sZero)
		}
	}
}

func TestRunMetrics_JSON_WithSerenaSync(t *testing.T) {
	rm := RunMetrics{
		RunID: "run-sync",
		SerenaSync: &SerenaSyncMetrics{
			Status:     "success",
			DurationMs: 5000,
			TokensIn:   100,
			TokensOut:  50,
			CostUSD:    0.02,
		},
	}
	data, err := json.Marshal(rm)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"serena_sync"`) {
		t.Errorf("JSON should contain serena_sync when populated: %s", s)
	}
	// Verify all nested field names (AC #6)
	for _, field := range []string{`"status"`, `"duration_ms"`, `"tokens_input"`, `"tokens_output"`, `"cost_usd"`} {
		if !strings.Contains(s, field) {
			t.Errorf("JSON serena_sync missing field %s: %s", field, s)
		}
	}
	// Verify field values
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	sync, ok := parsed["serena_sync"].(map[string]any)
	if !ok {
		t.Fatalf("serena_sync not a map: %v", parsed["serena_sync"])
	}
	if v, _ := sync["status"].(string); v != "success" {
		t.Errorf("status = %q, want %q", v, "success")
	}
	if v, _ := sync["duration_ms"].(float64); v != 5000 {
		t.Errorf("duration_ms = %v, want 5000", v)
	}
	if v, _ := sync["tokens_input"].(float64); v != 100 {
		t.Errorf("tokens_input = %v, want 100", v)
	}
	if v, _ := sync["tokens_output"].(float64); v != 50 {
		t.Errorf("tokens_output = %v, want 50", v)
	}
	if v, _ := sync["cost_usd"].(float64); v != 0.02 {
		t.Errorf("cost_usd = %v, want 0.02", v)
	}
}

func TestRunMetrics_JSON_WithoutSerenaSync(t *testing.T) {
	rm := RunMetrics{RunID: "run-nosync"}
	data, err := json.Marshal(rm)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s := string(data)
	if strings.Contains(s, `"serena_sync"`) {
		t.Errorf("JSON should omit serena_sync when nil: %s", s)
	}
}

func TestMetricsCollector_RecordSerenaSync_WithResult(t *testing.T) {
	mc := NewMetricsCollector("run-sync", nil)
	result := &session.SessionResult{
		Metrics: &session.SessionMetrics{
			InputTokens:  800,
			OutputTokens: 300,
			CostUSD:      0.07,
		},
	}
	mc.RecordSerenaSync("success", 15000, result)
	rm := mc.Finish()

	if rm.SerenaSync == nil {
		t.Fatal("SerenaSync should be populated after RecordSerenaSync")
	}
	sm := rm.SerenaSync
	if sm.Status != "success" {
		t.Errorf("Status = %q, want %q", sm.Status, "success")
	}
	if sm.DurationMs != 15000 {
		t.Errorf("DurationMs = %d, want 15000", sm.DurationMs)
	}
	if sm.TokensIn != 800 {
		t.Errorf("TokensIn = %d, want 800", sm.TokensIn)
	}
	if sm.TokensOut != 300 {
		t.Errorf("TokensOut = %d, want 300", sm.TokensOut)
	}
	if sm.CostUSD != 0.07 {
		t.Errorf("CostUSD = %f, want 0.07", sm.CostUSD)
	}
}

func TestMetricsCollector_RecordSerenaSync_NilResult(t *testing.T) {
	mc := NewMetricsCollector("run-nilres", nil)
	mc.RecordSerenaSync("skipped", 500, nil)
	rm := mc.Finish()

	if rm.SerenaSync == nil {
		t.Fatal("SerenaSync should be populated even with nil result")
	}
	sm := rm.SerenaSync
	// "skipped" is not a fail status, so derived aggregate = "success"
	if sm.Status != "success" {
		t.Errorf("Status = %q, want %q", sm.Status, "success")
	}
	if sm.DurationMs != 500 {
		t.Errorf("DurationMs = %d, want 500", sm.DurationMs)
	}
	if sm.TokensIn != 0 {
		t.Errorf("TokensIn = %d, want 0", sm.TokensIn)
	}
	if sm.TokensOut != 0 {
		t.Errorf("TokensOut = %d, want 0", sm.TokensOut)
	}
	if sm.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", sm.CostUSD)
	}
}

func TestMetricsCollector_RecordSerenaSync_NilReceiver(t *testing.T) {
	var mc *MetricsCollector
	// Must not panic (AC #4)
	mc.RecordSerenaSync("failed", 1000, nil)
}

func TestMetricsCollector_Finish_PreservesSerenaSync(t *testing.T) {
	mc := NewMetricsCollector("run-preserve", nil)
	mc.StartTask("task-1")
	mc.RecordSerenaSync("rollback", 8000, &session.SessionResult{
		Metrics: &session.SessionMetrics{
			InputTokens:  400,
			OutputTokens: 150,
			CostUSD:      0.03,
		},
	})
	mc.FinishTask("completed", "abc123")
	rm := mc.Finish()

	// Verify SerenaSync is preserved alongside task data
	if rm.SerenaSync == nil {
		t.Fatal("SerenaSync should be preserved in Finish()")
	}
	// "rollback" is a fail status, so single call derived aggregate = "failed"
	if rm.SerenaSync.Status != "failed" {
		t.Errorf("Status = %q, want %q", rm.SerenaSync.Status, "failed")
	}
	if rm.SerenaSync.TokensIn != 400 {
		t.Errorf("TokensIn = %d, want 400", rm.SerenaSync.TokensIn)
	}
	if rm.SerenaSync.DurationMs != 8000 {
		t.Errorf("DurationMs = %d, want 8000", rm.SerenaSync.DurationMs)
	}
	if rm.SerenaSync.TokensOut != 150 {
		t.Errorf("TokensOut = %d, want 150", rm.SerenaSync.TokensOut)
	}
	if rm.SerenaSync.CostUSD != 0.03 {
		t.Errorf("CostUSD = %f, want 0.03", rm.SerenaSync.CostUSD)
	}
	if rm.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %d, want 1", rm.TasksCompleted)
	}
}

// TestMetricsCollector_RecordSerenaSync_MultipleCalls verifies accumulation across
// multiple RecordSerenaSync calls: duration/tokens/cost are summed, status follows
// ternary logic (all success → "success", mixed → "partial", all fail → "failed").
func TestMetricsCollector_RecordSerenaSync_MultipleCalls(t *testing.T) {
	mc := NewMetricsCollector("run-multi", nil)

	// Call 1: success with metrics
	mc.RecordSerenaSync("success", 5000, &session.SessionResult{
		Metrics: &session.SessionMetrics{
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.01,
		},
	})

	// Call 2: failed with metrics
	mc.RecordSerenaSync("failed", 3000, &session.SessionResult{
		Metrics: &session.SessionMetrics{
			InputTokens:  200,
			OutputTokens: 80,
			CostUSD:      0.02,
		},
	})

	// Call 3: success with nil result
	mc.RecordSerenaSync("success", 2000, nil)

	rm := mc.Finish()
	if rm.SerenaSync == nil {
		t.Fatal("SerenaSync should be populated after multiple calls")
	}
	sm := rm.SerenaSync

	// Status: 1 fail out of 3 → "partial"
	if sm.Status != "partial" {
		t.Errorf("Status = %q, want %q (1 fail of 3 = partial)", sm.Status, "partial")
	}
	// Duration accumulated: 5000 + 3000 + 2000
	if sm.DurationMs != 10000 {
		t.Errorf("DurationMs = %d, want 10000", sm.DurationMs)
	}
	// Tokens accumulated from calls 1+2 (call 3 nil result)
	if sm.TokensIn != 300 {
		t.Errorf("TokensIn = %d, want 300 (100+200)", sm.TokensIn)
	}
	if sm.TokensOut != 130 {
		t.Errorf("TokensOut = %d, want 130 (50+80)", sm.TokensOut)
	}
	if sm.CostUSD != 0.03 {
		t.Errorf("CostUSD = %f, want 0.03 (0.01+0.02)", sm.CostUSD)
	}
}

// --- DESIGN-6: Findings progression and hydra detection ---

func TestMetricsCollector_RecordFindingsCycle_Progression(t *testing.T) {
	mc := NewMetricsCollector("run-prog", nil)
	mc.StartTask("task-1")

	// Cycle 1: 5 findings — first cycle, no hydra
	hydra := mc.RecordFindingsCycle(5)
	if hydra {
		t.Error("cycle 1: unexpected hydra detection")
	}

	// Cycle 2: 3 findings — decreasing, no hydra
	hydra = mc.RecordFindingsCycle(3)
	if hydra {
		t.Error("cycle 2: unexpected hydra on decreasing findings")
	}

	// Cycle 3: 4 findings — increasing, hydra detected
	hydra = mc.RecordFindingsCycle(4)
	if !hydra {
		t.Error("cycle 3: expected hydra detection on increase")
	}

	mc.FinishTask("completed", "abc")
	rm := mc.Finish()

	if len(rm.Tasks) != 1 {
		t.Fatalf("Tasks len = %d, want 1", len(rm.Tasks))
	}
	prog := rm.Tasks[0].FindingsProgression
	if len(prog) != 3 {
		t.Fatalf("FindingsProgression len = %d, want 3", len(prog))
	}
	if prog[0] != 5 || prog[1] != 3 || prog[2] != 4 {
		t.Errorf("FindingsProgression = %v, want [5 3 4]", prog)
	}
}

func TestMetricsCollector_RecordFindingsCycle_SameCountIsHydra(t *testing.T) {
	mc := NewMetricsCollector("run-same", nil)
	mc.StartTask("task-same")

	mc.RecordFindingsCycle(3)
	hydra := mc.RecordFindingsCycle(3) // same count = not decreasing
	if !hydra {
		t.Error("same count should be detected as hydra")
	}

	mc.FinishTask("completed", "def")
	rm := mc.Finish()
	prog := rm.Tasks[0].FindingsProgression
	if len(prog) != 2 || prog[0] != 3 || prog[1] != 3 {
		t.Errorf("FindingsProgression = %v, want [3 3]", prog)
	}
}

func TestMetricsCollector_RecordFindingsCycle_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-no-task", nil)
	// Should not panic and return false
	hydra := mc.RecordFindingsCycle(5)
	if hydra {
		t.Error("expected false when no task started")
	}
}

// --- RecordCycleDuration ---

func TestMetricsCollector_RecordCycleDuration_Accumulated(t *testing.T) {
	mc := NewMetricsCollector("run-cycle", nil)
	mc.StartTask("task-cycle")
	mc.RecordCycleDuration(1500)
	mc.RecordCycleDuration(2300)
	mc.RecordCycleDuration(900)
	mc.FinishTask("completed", "abc123")
	rm := mc.Finish()

	if len(rm.Tasks) != 1 {
		t.Fatalf("Tasks count = %d, want 1", len(rm.Tasks))
	}
	durations := rm.Tasks[0].CycleDurationsMs
	if len(durations) != 3 {
		t.Fatalf("CycleDurationsMs length = %d, want 3", len(durations))
	}
	want := []int64{1500, 2300, 900}
	for i, d := range durations {
		if d != want[i] {
			t.Errorf("CycleDurationsMs[%d] = %d, want %d", i, d, want[i])
		}
	}
}

func TestMetricsCollector_RecordCycleDuration_NoTask(t *testing.T) {
	mc := NewMetricsCollector("run-no-task", nil)
	// Should not panic when no task started
	mc.RecordCycleDuration(1000)
}

// TestReviewFinding_JSON_AgentOmitempty verifies Agent field JSON serialization (AC#1).
func TestReviewFinding_JSON_AgentOmitempty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		finding   ReviewFinding
		wantAgent bool
		wantValue string
	}{
		{
			name:      "agent present in JSON",
			finding:   ReviewFinding{Severity: "HIGH", Description: "test", Agent: "quality"},
			wantAgent: true,
			wantValue: "quality",
		},
		{
			name:      "agent omitted when empty",
			finding:   ReviewFinding{Severity: "LOW", Description: "test"},
			wantAgent: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.finding)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			s := string(data)
			if tc.wantAgent {
				if !strings.Contains(s, `"agent"`) {
					t.Errorf("JSON missing agent field: %s", s)
				}
				if !strings.Contains(s, tc.wantValue) {
					t.Errorf("JSON missing agent value %q: %s", tc.wantValue, s)
				}
			} else {
				if strings.Contains(s, `"agent"`) {
					t.Errorf("JSON should omit empty agent: %s", s)
				}
			}
		})
	}
}

// TestAgentFindingStats_JSON verifies JSON tags on AgentFindingStats (AC#6).
func TestAgentFindingStats_JSON(t *testing.T) {
	t.Parallel()
	stats := AgentFindingStats{Critical: 1, High: 2, Medium: 3, Low: 4}
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(data)
	for _, key := range []string{`"critical":1`, `"high":2`, `"medium":3`, `"low":4`} {
		if !strings.Contains(s, key) {
			t.Errorf("JSON missing %s: %s", key, s)
		}
	}
}

// TestMetricsCollector_RecordAgentFinding_Accumulation verifies per-agent severity counting (AC#8).
func TestMetricsCollector_RecordAgentFinding_Accumulation(t *testing.T) {
	t.Parallel()
	mc := NewMetricsCollector("run-agent", nil)

	mc.RecordAgentFinding("implementation", "HIGH")
	mc.RecordAgentFinding("implementation", "HIGH")
	mc.RecordAgentFinding("implementation", "HIGH")
	mc.RecordAgentFinding("quality", "MEDIUM")
	mc.RecordAgentFinding("quality", "MEDIUM")
	mc.RecordAgentFinding("quality", "LOW")
	mc.RecordAgentFinding("test-coverage", "CRITICAL")

	// Verify implementation stats
	impl := mc.agentStats["implementation"]
	if impl == nil {
		t.Fatal("agentStats[\"implementation\"] is nil")
	}
	if impl.High != 3 {
		t.Errorf("implementation.High = %d, want 3", impl.High)
	}
	if impl.Critical != 0 || impl.Medium != 0 || impl.Low != 0 {
		t.Errorf("implementation unexpected: C=%d M=%d L=%d", impl.Critical, impl.Medium, impl.Low)
	}

	// Verify quality stats
	qual := mc.agentStats["quality"]
	if qual == nil {
		t.Fatal("agentStats[\"quality\"] is nil")
	}
	if qual.Medium != 2 {
		t.Errorf("quality.Medium = %d, want 2", qual.Medium)
	}
	if qual.Low != 1 {
		t.Errorf("quality.Low = %d, want 1", qual.Low)
	}

	// Verify test-coverage stats
	tc := mc.agentStats["test-coverage"]
	if tc == nil {
		t.Fatal("agentStats[\"test-coverage\"] is nil")
	}
	if tc.Critical != 1 {
		t.Errorf("test-coverage.Critical = %d, want 1", tc.Critical)
	}
}

// TestMetricsCollector_RecordAgentFinding_EmptyAgent verifies empty agent -> "unknown" (AC#9).
func TestMetricsCollector_RecordAgentFinding_EmptyAgent(t *testing.T) {
	t.Parallel()
	mc := NewMetricsCollector("run-unknown", nil)

	mc.RecordAgentFinding("", "HIGH")
	mc.RecordAgentFinding("", "MEDIUM")

	unk := mc.agentStats["unknown"]
	if unk == nil {
		t.Fatal("agentStats[\"unknown\"] is nil")
	}
	if unk.High != 1 {
		t.Errorf("unknown.High = %d, want 1", unk.High)
	}
	if unk.Medium != 1 {
		t.Errorf("unknown.Medium = %d, want 1", unk.Medium)
	}
}

// TestMetricsCollector_RecordAgentFinding_NilReceiver verifies nil receiver no-op.
func TestMetricsCollector_RecordAgentFinding_NilReceiver(t *testing.T) {
	t.Parallel()
	var mc *MetricsCollector
	// Should not panic
	mc.RecordAgentFinding("quality", "HIGH")
}

// TestMetricsCollector_RecordAgentFinding_CaseInsensitive verifies severity case handling.
func TestMetricsCollector_RecordAgentFinding_CaseInsensitive(t *testing.T) {
	t.Parallel()
	mc := NewMetricsCollector("run-case", nil)
	mc.RecordAgentFinding("agent", "high")
	mc.RecordAgentFinding("agent", "Medium")
	mc.RecordAgentFinding("agent", "low")
	mc.RecordAgentFinding("agent", "CRITICAL")

	s := mc.agentStats["agent"]
	if s == nil {
		t.Fatal("agentStats[\"agent\"] is nil")
	}
	if s.Critical != 1 {
		t.Errorf("Critical = %d, want 1", s.Critical)
	}
	if s.High != 1 {
		t.Errorf("High = %d, want 1", s.High)
	}
	if s.Medium != 1 {
		t.Errorf("Medium = %d, want 1", s.Medium)
	}
	if s.Low != 1 {
		t.Errorf("Low = %d, want 1", s.Low)
	}
}

// TestMetricsCollector_Finish_AgentStats verifies AgentStats in Finish() output (AC#7).
func TestMetricsCollector_Finish_AgentStats(t *testing.T) {
	t.Parallel()
	mc := NewMetricsCollector("run-finish-agents", nil)

	mc.RecordAgentFinding("quality", "HIGH")
	mc.RecordAgentFinding("quality", "MEDIUM")
	mc.RecordAgentFinding("implementation", "LOW")

	mc.StartTask("task-1")
	mc.FinishTask("completed", "abc")

	rm := mc.Finish()

	if rm.AgentStats == nil {
		t.Fatal("AgentStats is nil")
	}
	if len(rm.AgentStats) != 2 {
		t.Fatalf("len(AgentStats) = %d, want 2", len(rm.AgentStats))
	}
	qual := rm.AgentStats["quality"]
	if qual == nil {
		t.Fatal("AgentStats[\"quality\"] is nil")
	}
	if qual.High != 1 {
		t.Errorf("quality.High = %d, want 1", qual.High)
	}
	if qual.Medium != 1 {
		t.Errorf("quality.Medium = %d, want 1", qual.Medium)
	}
	impl := rm.AgentStats["implementation"]
	if impl == nil {
		t.Fatal("AgentStats[\"implementation\"] is nil")
	}
	if impl.Low != 1 {
		t.Errorf("implementation.Low = %d, want 1", impl.Low)
	}
}

// TestMetricsCollector_Finish_AgentStats_Nil verifies nil AgentStats when no findings (omitempty).
func TestMetricsCollector_Finish_AgentStats_Nil(t *testing.T) {
	t.Parallel()
	mc := NewMetricsCollector("run-no-agents", nil)
	mc.StartTask("task-1")
	mc.FinishTask("completed", "abc")
	rm := mc.Finish()
	if rm.AgentStats != nil {
		t.Errorf("AgentStats = %v, want nil", rm.AgentStats)
	}
}

func TestMetricsCollector_RecordSession_ContextMetrics(t *testing.T) {
	mc := NewMetricsCollector("run-ctx", nil)
	mc.StartTask("task-1")

	// Session 1: no compactions, 30% fill.
	mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "", "execute", 1000, 0, 30.0)
	// Session 2: 1 compaction, 55% fill.
	mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "", "execute", 1000, 1, 55.0)
	// Session 3: no compactions, 42% fill.
	mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "", "execute", 1000, 0, 42.0)

	mc.FinishTask("completed", "sha1")
	rm := mc.Finish()

	task := rm.Tasks[0]
	if task.TotalCompactions != 1 {
		t.Errorf("TotalCompactions = %d, want 1", task.TotalCompactions)
	}
	if task.MaxContextFillPct != 55.0 {
		t.Errorf("MaxContextFillPct = %f, want 55.0", task.MaxContextFillPct)
	}
}

func TestMetricsCollector_RecordSession_ContextMetrics_NilCollector(t *testing.T) {
	mc := NewMetricsCollector("run-nil", nil)
	// No StartTask — current is nil.
	resolved := mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "sonnet", "execute", 1000, 2, 50.0)
	if resolved != "sonnet" {
		t.Errorf("resolved = %q, want %q", resolved, "sonnet")
	}
	// Should not panic.
}

func TestMetricsCollector_Finish_ContextAggregation(t *testing.T) {
	mc := NewMetricsCollector("run-agg", nil)

	mc.StartTask("task-1")
	mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "", "execute", 1000, 1, 55.0)
	mc.FinishTask("completed", "sha1")

	mc.StartTask("task-2")
	mc.RecordSession(&session.SessionMetrics{InputTokens: 100}, "", "execute", 1000, 3, 42.0)
	mc.FinishTask("completed", "sha2")

	rm := mc.Finish()

	if rm.TotalCompactions != 4 {
		t.Errorf("RunMetrics.TotalCompactions = %d, want 4", rm.TotalCompactions)
	}
	if rm.MaxContextFillPct != 55.0 {
		t.Errorf("RunMetrics.MaxContextFillPct = %f, want 55.0", rm.MaxContextFillPct)
	}
}
