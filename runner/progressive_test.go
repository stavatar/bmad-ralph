package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeverityLevel_Ordering(t *testing.T) {
	if SeverityLow >= SeverityMedium {
		t.Errorf("SeverityLow (%d) should be < SeverityMedium (%d)", SeverityLow, SeverityMedium)
	}
	if SeverityMedium >= SeverityHigh {
		t.Errorf("SeverityMedium (%d) should be < SeverityHigh (%d)", SeverityMedium, SeverityHigh)
	}
	if SeverityHigh >= SeverityCritical {
		t.Errorf("SeverityHigh (%d) should be < SeverityCritical (%d)", SeverityHigh, SeverityCritical)
	}
}

func TestSeverityLevel_Values(t *testing.T) {
	tests := []struct {
		name string
		got  SeverityLevel
		want int
	}{
		{"SeverityLow", SeverityLow, 0},
		{"SeverityMedium", SeverityMedium, 1},
		{"SeverityHigh", SeverityHigh, 2},
		{"SeverityCritical", SeverityCritical, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.got) != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestSeverityLevel_String(t *testing.T) {
	tests := []struct {
		name  string
		level SeverityLevel
		want  string
	}{
		{"LOW", SeverityLow, "LOW"},
		{"MEDIUM", SeverityMedium, "MEDIUM"},
		{"HIGH", SeverityHigh, "HIGH"},
		{"CRITICAL", SeverityCritical, "CRITICAL"},
		{"unknown", SeverityLevel(99), "SeverityLevel(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.want {
				t.Errorf("SeverityLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestParseSeverity_AllValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  SeverityLevel
	}{
		{"uppercase LOW", "LOW", SeverityLow},
		{"uppercase MEDIUM", "MEDIUM", SeverityMedium},
		{"uppercase HIGH", "HIGH", SeverityHigh},
		{"uppercase CRITICAL", "CRITICAL", SeverityCritical},
		{"lowercase low", "low", SeverityLow},
		{"lowercase medium", "medium", SeverityMedium},
		{"lowercase high", "high", SeverityHigh},
		{"lowercase critical", "critical", SeverityCritical},
		{"mixed case Low", "Low", SeverityLow},
		{"mixed case Medium", "Medium", SeverityMedium},
		{"mixed case High", "High", SeverityHigh},
		{"mixed case Critical", "Critical", SeverityCritical},
		{"empty string", "", SeverityLow},
		{"unknown value", "UNKNOWN", SeverityLow},
		{"random string", "foo", SeverityLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSeverity(tt.input)
			if got != tt.want {
				t.Errorf("ParseSeverity(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestProgressiveParams_DefaultSixCycles(t *testing.T) {
	tests := []struct {
		name            string
		cycle           int
		wantSeverity    SeverityLevel
		wantMaxFindings int
		wantIncremental bool
		wantHighEffort  bool
	}{
		{"cycle 1", 1, SeverityLow, 5, false, false},
		{"cycle 2", 2, SeverityLow, 5, false, false},
		{"cycle 3", 3, SeverityMedium, 3, true, true},
		{"cycle 4", 4, SeverityHigh, 1, true, true},
		{"cycle 5", 5, SeverityCritical, 1, true, true},
		{"cycle 6", 6, SeverityCritical, 1, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProgressiveParams(tt.cycle, 6)
			if got.MinSeverity != tt.wantSeverity {
				t.Errorf("MinSeverity = %d, want %d", got.MinSeverity, tt.wantSeverity)
			}
			if got.MaxFindings != tt.wantMaxFindings {
				t.Errorf("MaxFindings = %d, want %d", got.MaxFindings, tt.wantMaxFindings)
			}
			if got.IncrementalDiff != tt.wantIncremental {
				t.Errorf("IncrementalDiff = %v, want %v", got.IncrementalDiff, tt.wantIncremental)
			}
			if got.HighEffort != tt.wantHighEffort {
				t.Errorf("HighEffort = %v, want %v", got.HighEffort, tt.wantHighEffort)
			}
		})
	}
}

func TestProgressiveParams_ThreeCycles(t *testing.T) {
	tests := []struct {
		name            string
		cycle           int
		wantSeverity    SeverityLevel
		wantMaxFindings int
		wantIncremental bool
		wantHighEffort  bool
	}{
		{"cycle 1 of 3", 1, SeverityLow, 5, false, false},
		{"cycle 2 of 3", 2, SeverityMedium, 3, true, true},
		{"cycle 3 of 3", 3, SeverityCritical, 1, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProgressiveParams(tt.cycle, 3)
			if got.MinSeverity != tt.wantSeverity {
				t.Errorf("MinSeverity = %d, want %d", got.MinSeverity, tt.wantSeverity)
			}
			if got.MaxFindings != tt.wantMaxFindings {
				t.Errorf("MaxFindings = %d, want %d", got.MaxFindings, tt.wantMaxFindings)
			}
			if got.IncrementalDiff != tt.wantIncremental {
				t.Errorf("IncrementalDiff = %v, want %v", got.IncrementalDiff, tt.wantIncremental)
			}
			if got.HighEffort != tt.wantHighEffort {
				t.Errorf("HighEffort = %v, want %v", got.HighEffort, tt.wantHighEffort)
			}
		})
	}
}

func TestProgressiveParams_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		cycle           int
		maxCycles       int
		wantSeverity    SeverityLevel
		wantMaxFindings int
		wantIncremental bool
		wantHighEffort  bool
	}{
		{"cycle 0 clamped to 1", 0, 6, SeverityLow, 5, false, false},
		{"cycle beyond max clamped", 7, 6, SeverityCritical, 1, true, true},
		{"single cycle max", 1, 1, SeverityCritical, 1, true, true},
		{"negative cycle clamped", -5, 6, SeverityLow, 5, false, false},
		{"large cycle clamped", 100, 6, SeverityCritical, 1, true, true},
		{"maxCycles zero", 1, 0, SeverityCritical, 1, true, true},
		{"maxCycles negative", 1, -1, SeverityCritical, 1, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProgressiveParams(tt.cycle, tt.maxCycles)
			if got.MinSeverity != tt.wantSeverity {
				t.Errorf("MinSeverity = %d, want %d", got.MinSeverity, tt.wantSeverity)
			}
			if got.MaxFindings != tt.wantMaxFindings {
				t.Errorf("MaxFindings = %d, want %d", got.MaxFindings, tt.wantMaxFindings)
			}
			if got.IncrementalDiff != tt.wantIncremental {
				t.Errorf("IncrementalDiff = %v, want %v", got.IncrementalDiff, tt.wantIncremental)
			}
			if got.HighEffort != tt.wantHighEffort {
				t.Errorf("HighEffort = %v, want %v", got.HighEffort, tt.wantHighEffort)
			}
		})
	}
}

// --- FilterBySeverity tests ---

func TestFilterBySeverity_AllThresholds(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "LOW", Description: "low issue", File: "a.go", Line: 1},
		{Severity: "MEDIUM", Description: "medium issue", File: "b.go", Line: 2},
		{Severity: "HIGH", Description: "high issue", File: "c.go", Line: 3},
		{Severity: "CRITICAL", Description: "critical issue", File: "d.go", Line: 4},
	}

	tests := []struct {
		name         string
		minSeverity  SeverityLevel
		wantCount    int
		wantSeverities []string
	}{
		{"LOW threshold passes all", SeverityLow, 4, []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}},
		{"MEDIUM threshold", SeverityMedium, 3, []string{"MEDIUM", "HIGH", "CRITICAL"}},
		{"HIGH threshold", SeverityHigh, 2, []string{"HIGH", "CRITICAL"}},
		{"CRITICAL threshold", SeverityCritical, 1, []string{"CRITICAL"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterBySeverity(findings, tt.minSeverity)
			if len(got) != tt.wantCount {
				t.Fatalf("FilterBySeverity() returned %d findings, want %d", len(got), tt.wantCount)
			}
			for i, f := range got {
				if f.Severity != tt.wantSeverities[i] {
					t.Errorf("finding[%d].Severity = %q, want %q", i, f.Severity, tt.wantSeverities[i])
				}
				if f.Description == "" {
					t.Errorf("finding[%d].Description is empty", i)
				}
				if f.File == "" {
					t.Errorf("finding[%d].File is empty", i)
				}
				if f.Line == 0 {
					t.Errorf("finding[%d].Line is 0", i)
				}
			}
		})
	}
}

func TestFilterBySeverity_EmptyInput(t *testing.T) {
	got := FilterBySeverity(nil, SeverityMedium)
	if len(got) != 0 {
		t.Errorf("FilterBySeverity(nil) returned %d findings, want 0", len(got))
	}
	got = FilterBySeverity([]ReviewFinding{}, SeverityLow)
	if len(got) != 0 {
		t.Errorf("FilterBySeverity([]) returned %d findings, want 0", len(got))
	}
}

func TestFilterBySeverity_DoesNotModifyInput(t *testing.T) {
	original := []ReviewFinding{
		{Severity: "LOW", Description: "low"},
		{Severity: "HIGH", Description: "high"},
	}
	_ = FilterBySeverity(original, SeverityHigh)
	if len(original) != 2 {
		t.Fatalf("input slice modified: len = %d, want 2", len(original))
	}
	if original[0].Severity != "LOW" {
		t.Errorf("input[0].Severity changed to %q", original[0].Severity)
	}
}

// --- TruncateFindings tests ---

func TestTruncateFindings_SortBySeverity(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "LOW", Description: "low issue"},
		{Severity: "CRITICAL", Description: "critical issue"},
		{Severity: "MEDIUM", Description: "medium issue"},
	}
	got := TruncateFindings(findings, 2)
	if len(got) != 2 {
		t.Fatalf("TruncateFindings() returned %d findings, want 2", len(got))
	}
	if got[0].Severity != "CRITICAL" {
		t.Errorf("got[0].Severity = %q, want CRITICAL", got[0].Severity)
	}
	if got[0].Description != "critical issue" {
		t.Errorf("got[0].Description = %q, want %q", got[0].Description, "critical issue")
	}
	if got[1].Severity != "MEDIUM" {
		t.Errorf("got[1].Severity = %q, want MEDIUM", got[1].Severity)
	}
	if got[1].Description != "medium issue" {
		t.Errorf("got[1].Description = %q, want %q", got[1].Description, "medium issue")
	}
}

func TestTruncateFindings_NoTruncationWhenUnderBudget(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "HIGH", Description: "high"},
		{Severity: "LOW", Description: "low"},
	}
	got := TruncateFindings(findings, 5)
	if len(got) != 2 {
		t.Fatalf("TruncateFindings() returned %d findings, want 2", len(got))
	}
}

func TestTruncateFindings_ExactBudget(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "HIGH", Description: "high"},
		{Severity: "LOW", Description: "low"},
	}
	got := TruncateFindings(findings, 2)
	if len(got) != 2 {
		t.Fatalf("TruncateFindings() returned %d findings, want 2", len(got))
	}
}

func TestTruncateFindings_EmptyInput(t *testing.T) {
	got := TruncateFindings(nil, 3)
	if len(got) != 0 {
		t.Errorf("TruncateFindings(nil) returned %d findings, want 0", len(got))
	}
}

func TestTruncateFindings_BudgetOne(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "LOW", Description: "low"},
		{Severity: "CRITICAL", Description: "critical"},
		{Severity: "MEDIUM", Description: "medium"},
	}
	got := TruncateFindings(findings, 1)
	if len(got) != 1 {
		t.Fatalf("TruncateFindings() returned %d findings, want 1", len(got))
	}
	if got[0].Severity != "CRITICAL" {
		t.Errorf("got[0].Severity = %q, want CRITICAL (highest severity kept)", got[0].Severity)
	}
}

func TestTruncateFindings_MaxCountZero(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "HIGH", Description: "issue"},
	}
	got := TruncateFindings(findings, 0)
	if len(got) != 0 {
		t.Errorf("TruncateFindings(_, 0) returned %d findings, want 0", len(got))
	}
}

func TestTruncateFindings_DoesNotModifyInput(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "LOW", Description: "low issue"},
		{Severity: "CRITICAL", Description: "critical issue"},
		{Severity: "MEDIUM", Description: "medium issue"},
	}
	// Save original order.
	origFirst := findings[0].Severity
	origSecond := findings[1].Severity
	origThird := findings[2].Severity

	got := TruncateFindings(findings, 1)
	if len(got) != 1 {
		t.Fatalf("TruncateFindings() returned %d, want 1", len(got))
	}
	if got[0].Severity != "CRITICAL" {
		t.Errorf("got[0].Severity = %q, want CRITICAL", got[0].Severity)
	}
	// Input slice must not be reordered.
	if findings[0].Severity != origFirst {
		t.Errorf("input[0].Severity mutated: got %q, was %q", findings[0].Severity, origFirst)
	}
	if findings[1].Severity != origSecond {
		t.Errorf("input[1].Severity mutated: got %q, was %q", findings[1].Severity, origSecond)
	}
	if findings[2].Severity != origThird {
		t.Errorf("input[2].Severity mutated: got %q, was %q", findings[2].Severity, origThird)
	}
}

// --- writeFilteredFindings tests ---

func TestWriteFilteredFindings_Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review-findings.md")
	findings := []ReviewFinding{
		{Severity: "CRITICAL", Description: "null pointer"},
		{Severity: "HIGH", Description: "buffer overflow"},
	}
	err := writeFilteredFindings(path, findings)
	if err != nil {
		t.Fatalf("writeFilteredFindings() error: %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "### [CRITICAL] null pointer") {
		t.Errorf("content missing CRITICAL finding: %q", content)
	}
	if !strings.Contains(content, "### [HIGH] buffer overflow") {
		t.Errorf("content missing HIGH finding: %q", content)
	}
	if strings.Count(content, "### [") != 2 {
		t.Errorf("expected 2 findings headers, got %d in: %q", strings.Count(content, "### ["), content)
	}
}

func TestWriteFilteredFindings_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review-findings.md")
	err := writeFilteredFindings(path, nil)
	if err != nil {
		t.Fatalf("writeFilteredFindings() error: %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %q", string(data))
	}
}
