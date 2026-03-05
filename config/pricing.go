package config

// Pricing holds per-model token pricing rates (USD per 1M tokens).
type Pricing struct {
	InputPer1M  float64 `yaml:"input_per_1m" json:"input_per_1m"`
	OutputPer1M float64 `yaml:"output_per_1m" json:"output_per_1m"`
	CachePer1M  float64 `yaml:"cache_per_1m" json:"cache_per_1m"`
}

// DefaultPricing contains hardcoded baseline pricing for known Anthropic models.
// Users can override per-model via config.yaml model_pricing field.
var DefaultPricing = map[string]Pricing{
	"claude-sonnet-4-20250514": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	"claude-opus-4-20250514":   {InputPer1M: 15.0, OutputPer1M: 75.0, CachePer1M: 1.50},
}

// MergePricing returns a merged pricing map: defaults overridden by overrides.
func MergePricing(defaults, overrides map[string]Pricing) map[string]Pricing {
	merged := make(map[string]Pricing, len(defaults)+len(overrides))
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// MostExpensiveModel returns the model key with the highest OutputPer1M price.
// Returns empty string if pricing map is empty.
func MostExpensiveModel(pricing map[string]Pricing) string {
	var maxModel string
	var maxPrice float64
	for k, v := range pricing {
		if maxModel == "" || v.OutputPer1M > maxPrice {
			maxModel = k
			maxPrice = v.OutputPer1M
		}
	}
	return maxModel
}
