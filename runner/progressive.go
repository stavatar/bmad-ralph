package runner

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// SeverityLevel represents the severity of a review finding.
// Levels are ordered: SeverityLow < SeverityMedium < SeverityHigh < SeverityCritical.
type SeverityLevel int

const (
	// SeverityLow is the lowest severity level.
	SeverityLow SeverityLevel = iota
	// SeverityMedium is a moderate severity level.
	SeverityMedium
	// SeverityHigh is a high severity level.
	SeverityHigh
	// SeverityCritical is the highest severity level.
	SeverityCritical
)

// String returns the string representation of a SeverityLevel.
func (s SeverityLevel) String() string {
	switch s {
	case SeverityLow:
		return "LOW"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityHigh:
		return "HIGH"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return fmt.Sprintf("SeverityLevel(%d)", int(s))
	}
}

// ParseSeverity converts a string to a SeverityLevel.
// Case-insensitive. Returns SeverityLow for unknown or empty input.
func ParseSeverity(s string) SeverityLevel {
	switch strings.ToUpper(s) {
	case "LOW":
		return SeverityLow
	case "MEDIUM":
		return SeverityMedium
	case "HIGH":
		return SeverityHigh
	case "CRITICAL":
		return SeverityCritical
	default:
		return SeverityLow
	}
}

// ProgressiveReviewParams holds review parameters for a specific cycle
// in the progressive review scheme.
type ProgressiveReviewParams struct {
	MinSeverity    SeverityLevel
	MaxFindings    int
	IncrementalDiff bool
	HighEffort     bool
}

// ProgressiveParams returns review parameters for the given cycle number
// within a run of maxCycles total cycles. The cycle is clamped to [1, maxCycles].
//
// For maxCycles=1, returns the strictest params (CRITICAL/1/true/true).
// For maxCycles>1, severity escalates proportionally:
//   - position < 0.33 -> LOW, maxFindings=5, incremental=false, highEffort=false
//   - position <= 0.50 -> MEDIUM, maxFindings=3, incremental=true, highEffort=true
//   - position < 0.67 -> HIGH, maxFindings=1, incremental=true, highEffort=true
//   - position >= 0.67 -> CRITICAL, maxFindings=1, incremental=true, highEffort=true
func ProgressiveParams(cycle, maxCycles int) ProgressiveReviewParams {
	if maxCycles <= 1 {
		return ProgressiveReviewParams{
			MinSeverity:    SeverityCritical,
			MaxFindings:    1,
			IncrementalDiff: true,
			HighEffort:     true,
		}
	}

	// Clamp cycle to [1, maxCycles].
	if cycle < 1 {
		cycle = 1
	}
	if cycle > maxCycles {
		cycle = maxCycles
	}

	position := float64(cycle-1) / float64(maxCycles-1)

	switch {
	case position < 0.33:
		return ProgressiveReviewParams{
			MinSeverity:    SeverityLow,
			MaxFindings:    5,
			IncrementalDiff: false,
			HighEffort:     false,
		}
	case position <= 0.50: // 0.50 inclusive to ensure cycle 2 of 3 maps to MEDIUM per AC
		return ProgressiveReviewParams{
			MinSeverity:    SeverityMedium,
			MaxFindings:    3,
			IncrementalDiff: true,
			HighEffort:     true,
		}
	case position < 0.67:
		return ProgressiveReviewParams{
			MinSeverity:    SeverityHigh,
			MaxFindings:    1,
			IncrementalDiff: true,
			HighEffort:     true,
		}
	default:
		return ProgressiveReviewParams{
			MinSeverity:    SeverityCritical,
			MaxFindings:    1,
			IncrementalDiff: true,
			HighEffort:     true,
		}
	}
}

// FilterBySeverity returns findings with severity >= minSeverity.
// Uses ParseSeverity to convert finding.Severity strings.
// Returns a new slice; the input is not modified.
func FilterBySeverity(findings []ReviewFinding, minSeverity SeverityLevel) []ReviewFinding {
	var result []ReviewFinding
	for _, f := range findings {
		if ParseSeverity(f.Severity) >= minSeverity {
			result = append(result, f)
		}
	}
	return result
}

// TruncateFindings sorts findings by severity descending and returns at most maxCount.
// If len(findings) <= maxCount, returns the input unchanged.
// Uses sort.SliceStable to preserve order among equal severities.
func TruncateFindings(findings []ReviewFinding, maxCount int) []ReviewFinding {
	if len(findings) <= maxCount {
		return findings
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return ParseSeverity(findings[i].Severity) > ParseSeverity(findings[j].Severity)
	})
	return findings[:maxCount]
}

// writeFilteredFindings rewrites review-findings.md with only the given findings.
// Each finding is written as "### [SEVERITY] Description" matching the format
// parsed by findingSeverityRe in DetermineReviewOutcome.
func writeFilteredFindings(path string, findings []ReviewFinding) error {
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "### [%s] %s\n\n", f.Severity, f.Description)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
