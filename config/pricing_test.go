package config

import "testing"

func TestDefaultPricing_KnownModels(t *testing.T) {
	tests := []struct {
		model       string
		wantInput   float64
		wantOutput  float64
		wantCache   float64
	}{
		{"claude-sonnet-4-20250514", 3.0, 15.0, 0.30},
		{"claude-opus-4-20250514", 15.0, 75.0, 1.50},
	}
	for _, tt := range tests {
		p, ok := DefaultPricing[tt.model]
		if !ok {
			t.Errorf("DefaultPricing missing model %q", tt.model)
			continue
		}
		if p.InputPer1M != tt.wantInput {
			t.Errorf("%s: InputPer1M = %f, want %f", tt.model, p.InputPer1M, tt.wantInput)
		}
		if p.OutputPer1M != tt.wantOutput {
			t.Errorf("%s: OutputPer1M = %f, want %f", tt.model, p.OutputPer1M, tt.wantOutput)
		}
		if p.CachePer1M != tt.wantCache {
			t.Errorf("%s: CachePer1M = %f, want %f", tt.model, p.CachePer1M, tt.wantCache)
		}
	}
}

func TestMergePricing_Override(t *testing.T) {
	defaults := map[string]Pricing{
		"model-a": {InputPer1M: 1.0, OutputPer1M: 5.0, CachePer1M: 0.1},
		"model-b": {InputPer1M: 2.0, OutputPer1M: 10.0, CachePer1M: 0.2},
	}
	overrides := map[string]Pricing{
		"model-b": {InputPer1M: 99.0, OutputPer1M: 99.0, CachePer1M: 99.0},
		"model-c": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.3},
	}

	merged := MergePricing(defaults, overrides)

	// model-a: unchanged from defaults
	if merged["model-a"].InputPer1M != 1.0 {
		t.Errorf("model-a InputPer1M = %f, want 1.0", merged["model-a"].InputPer1M)
	}
	// model-b: overridden
	if merged["model-b"].InputPer1M != 99.0 {
		t.Errorf("model-b InputPer1M = %f, want 99.0", merged["model-b"].InputPer1M)
	}
	// model-c: added from overrides
	if merged["model-c"].OutputPer1M != 15.0 {
		t.Errorf("model-c OutputPer1M = %f, want 15.0", merged["model-c"].OutputPer1M)
	}
	// total count
	if len(merged) != 3 {
		t.Errorf("len(merged) = %d, want 3", len(merged))
	}
}

func TestMergePricing_EmptyOverrides(t *testing.T) {
	merged := MergePricing(DefaultPricing, nil)
	if len(merged) != len(DefaultPricing) {
		t.Errorf("len(merged) = %d, want %d", len(merged), len(DefaultPricing))
	}
}

func TestMergePricing_EmptyDefaults(t *testing.T) {
	overrides := map[string]Pricing{
		"custom": {InputPer1M: 1.0, OutputPer1M: 2.0, CachePer1M: 0.1},
	}
	merged := MergePricing(nil, overrides)
	if len(merged) != 1 {
		t.Errorf("len(merged) = %d, want 1", len(merged))
	}
	if merged["custom"].OutputPer1M != 2.0 {
		t.Errorf("custom OutputPer1M = %f, want 2.0", merged["custom"].OutputPer1M)
	}
}

func TestMostExpensiveModel(t *testing.T) {
	pricing := map[string]Pricing{
		"cheap":  {OutputPer1M: 5.0},
		"mid":    {OutputPer1M: 15.0},
		"pricey": {OutputPer1M: 75.0},
	}
	got := MostExpensiveModel(pricing)
	if got != "pricey" {
		t.Errorf("MostExpensiveModel = %q, want %q", got, "pricey")
	}
}

func TestMostExpensiveModel_Empty(t *testing.T) {
	got := MostExpensiveModel(map[string]Pricing{})
	if got != "" {
		t.Errorf("MostExpensiveModel(empty) = %q, want empty", got)
	}
}

func TestMostExpensiveModel_DefaultPricing(t *testing.T) {
	got := MostExpensiveModel(DefaultPricing)
	if got != "claude-opus-4-20250514" {
		t.Errorf("MostExpensiveModel(DefaultPricing) = %q, want %q", got, "claude-opus-4-20250514")
	}
}
