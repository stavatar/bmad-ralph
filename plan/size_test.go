package plan

import (
	"strings"
	"testing"
)

func TestCheckSize_Scenarios(t *testing.T) {
	tests := []struct {
		name       string
		inputs     []PlanInput
		wantWarn   bool
		wantEmpty  bool
		wantSubstr string // checked only when wantWarn == true
	}{
		{
			name:      "empty inputs",
			inputs:    nil,
			wantWarn:  false,
			wantEmpty: true,
		},
		{
			name: "below threshold",
			inputs: []PlanInput{
				{Content: make([]byte, 50_000)},
				{Content: make([]byte, 49_999)},
			},
			wantWarn:  false,
			wantEmpty: true,
		},
		{
			name: "exactly threshold",
			inputs: []PlanInput{
				{Content: make([]byte, 100_000)},
			},
			wantWarn:  false,
			wantEmpty: true,
		},
		{
			name: "above threshold",
			inputs: []PlanInput{
				{Content: make([]byte, 100_001)},
			},
			wantWarn:   true,
			wantEmpty:  false,
			wantSubstr: "shard-doc",
		},
		{
			name: "multiple inputs above threshold",
			inputs: []PlanInput{
				{Content: make([]byte, 60_000)},
				{Content: make([]byte, 60_000)},
			},
			wantWarn:   true,
			wantEmpty:  false,
			wantSubstr: "117KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, msg := CheckSize(tt.inputs)
			if warn != tt.wantWarn {
				t.Errorf("warn = %v, want %v", warn, tt.wantWarn)
			}
			if tt.wantEmpty && msg != "" {
				t.Errorf("msg = %q, want empty", msg)
			}
			if !tt.wantEmpty && msg == "" {
				t.Errorf("msg is empty, want non-empty")
			}
			if tt.wantSubstr != "" && !strings.Contains(msg, tt.wantSubstr) {
				t.Errorf("msg = %q, want substring %q", msg, tt.wantSubstr)
			}
		})
	}
}
