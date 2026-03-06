package runner

import "testing"

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
	if SeverityLow != 0 {
		t.Errorf("SeverityLow = %d, want 0", SeverityLow)
	}
	if SeverityMedium != 1 {
		t.Errorf("SeverityMedium = %d, want 1", SeverityMedium)
	}
	if SeverityHigh != 2 {
		t.Errorf("SeverityHigh = %d, want 2", SeverityHigh)
	}
	if SeverityCritical != 3 {
		t.Errorf("SeverityCritical = %d, want 3", SeverityCritical)
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
