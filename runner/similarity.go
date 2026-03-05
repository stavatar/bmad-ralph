package runner

// SimilarityDetector tracks sliding-window similarity between diff snapshots
// to detect when the loop is producing repetitive changes (similarity loop).
// Uses average pairwise Jaccard similarity across ALL pairs in the window.
type SimilarityDetector struct {
	window  int
	warnAt  float64
	hardAt  float64
	history [][]string
}

// NewSimilarityDetector creates a detector with the given sliding window size
// and warning/hard thresholds. Check returns ("", 0.0) when fewer than 2 entries exist.
func NewSimilarityDetector(window int, warnAt, hardAt float64) *SimilarityDetector {
	return &SimilarityDetector{
		window: window,
		warnAt: warnAt,
		hardAt: hardAt,
	}
}

// JaccardSimilarity computes |intersection| / |union| of two string slices
// treated as sets. Returns 0.0 for empty inputs.
func JaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	setA := make(map[string]struct{}, len(a))
	for _, s := range a {
		setA[s] = struct{}{}
	}

	setB := make(map[string]struct{}, len(b))
	for _, s := range b {
		setB[s] = struct{}{}
	}

	intersection := 0
	for s := range setA {
		if _, ok := setB[s]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

// Push adds a diff snapshot to the sliding window. Nil or empty slices are
// skipped (no-op) to avoid polluting the history with empty entries.
// When the window is full, the oldest entry is evicted (FIFO).
func (d *SimilarityDetector) Push(diffLines []string) {
	if len(diffLines) == 0 {
		return
	}
	d.history = append(d.history, diffLines)
	if len(d.history) > d.window {
		d.history = d.history[len(d.history)-d.window:]
	}
}

// Check computes the average pairwise Jaccard similarity across ALL pairs
// in the sliding window and returns a level and score.
//
// Returns:
//   - ("hard", score) if avg similarity > hardAt (checked first)
//   - ("warn", score) if avg similarity > warnAt
//   - ("", score) if diffs are diverse
//   - ("", 0.0) if fewer than 2 entries in the window
func (d *SimilarityDetector) Check() (string, float64) {
	n := len(d.history)
	if n < 2 {
		return "", 0.0
	}

	var total float64
	pairs := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			total += JaccardSimilarity(d.history[i], d.history[j])
			pairs++
		}
	}

	avg := total / float64(pairs)

	if avg > d.hardAt {
		return "hard", avg
	}
	if avg > d.warnAt {
		return "warn", avg
	}
	return "", avg
}
