package runner

import (
	"math"
	"testing"
)

func TestJaccardSimilarity_EdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a    []string
		b    []string
		want float64
	}{
		{
			name: "both empty",
			a:    []string{},
			b:    []string{},
			want: 0.0,
		},
		{
			name: "a empty",
			a:    []string{},
			b:    []string{"x"},
			want: 0.0,
		},
		{
			name: "b empty",
			a:    []string{"x"},
			b:    []string{},
			want: 0.0,
		},
		{
			name: "a nil",
			a:    nil,
			b:    []string{"x"},
			want: 0.0,
		},
		{
			name: "b nil",
			a:    []string{"x"},
			b:    nil,
			want: 0.0,
		},
		{
			name: "identical single",
			a:    []string{"a"},
			b:    []string{"a"},
			want: 1.0,
		},
		{
			name: "identical multi",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "c"},
			want: 1.0,
		},
		{
			name: "completely disjoint",
			a:    []string{"a", "b"},
			b:    []string{"c", "d"},
			want: 0.0,
		},
		{
			name: "partial overlap",
			a:    []string{"a", "b", "c"},
			b:    []string{"b", "c", "d"},
			// intersection={b,c}=2, union={a,b,c,d}=4 → 0.5
			want: 0.5,
		},
		{
			name: "subset",
			a:    []string{"a", "b"},
			b:    []string{"a", "b", "c"},
			// intersection=2, union=3 → 0.6667
			want: 2.0 / 3.0,
		},
		{
			name: "duplicates in input treated as set",
			a:    []string{"a", "a", "b"},
			b:    []string{"a", "b", "b"},
			// sets: {a,b} and {a,b} → 1.0
			want: 1.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := JaccardSimilarity(tc.a, tc.b)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("JaccardSimilarity(%v, %v) = %f, want %f", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSimilarityDetector_Push_EmptySlice(t *testing.T) {
	t.Parallel()
	d := NewSimilarityDetector(3, 0.85, 0.95)

	// nil → no-op
	d.Push(nil)
	level, score := d.Check()
	if level != "" || score != 0.0 {
		t.Errorf("after Push(nil): level=%q score=%f, want empty/0.0", level, score)
	}

	// empty slice → no-op
	d.Push([]string{})
	level, score = d.Check()
	if level != "" || score != 0.0 {
		t.Errorf("after Push([]string{}): level=%q score=%f, want empty/0.0", level, score)
	}

	// push one real entry, then nil — should still have only 1 entry (< 2)
	d.Push([]string{"pkg/a"})
	d.Push(nil)
	level, score = d.Check()
	if level != "" || score != 0.0 {
		t.Errorf("after 1 real + nil: level=%q score=%f, want empty/0.0", level, score)
	}
}

func TestSimilarityDetector_Push_WindowOverflow(t *testing.T) {
	t.Parallel()
	d := NewSimilarityDetector(2, 0.85, 0.95)

	d.Push([]string{"a"})
	d.Push([]string{"b"})
	d.Push([]string{"c"}) // should evict "a"

	// Window should contain ["b"] and ["c"] — disjoint → similarity 0.0
	level, score := d.Check()
	if level != "" {
		t.Errorf("level = %q, want empty (disjoint after overflow)", level)
	}
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0 (disjoint b vs c)", score)
	}
}

func TestSimilarityDetector_Check_Thresholds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		window    int
		warnAt    float64
		hardAt    float64
		entries   [][]string
		wantLevel string
		wantMin   float64
		wantMax   float64
	}{
		{
			name:      "insufficient data single entry",
			window:    3,
			warnAt:    0.85,
			hardAt:    0.95,
			entries:   [][]string{{"a"}},
			wantLevel: "",
			wantMin:   0.0,
			wantMax:   0.0,
		},
		{
			name:   "diverse diffs below warn",
			window: 3,
			warnAt: 0.85,
			hardAt: 0.95,
			entries: [][]string{
				{"a", "b"},
				{"c", "d"},
				{"e", "f"},
			},
			wantLevel: "",
			wantMin:   0.0,
			wantMax:   0.0,
		},
		{
			name:   "warn threshold boundary not exceeded",
			window: 3,
			warnAt: 0.5,
			hardAt: 0.95,
			entries: [][]string{
				{"a", "b", "c"},
				{"a", "b", "d"},
				{"a", "b", "e"},
			},
			// pairs: (0,1)=2/4=0.5, (0,2)=2/4=0.5, (1,2)=2/4=0.5 → avg=0.5
			// 0.5 == warnAt, but check is > warnAt, so NOT warn.
			// Use warnAt=0.4 to trigger warn.
			wantLevel: "",
			wantMin:   0.49,
			wantMax:   0.51,
		},
		{
			name:   "warn threshold triggered",
			window: 3,
			warnAt: 0.4,
			hardAt: 0.95,
			entries: [][]string{
				{"a", "b", "c"},
				{"a", "b", "d"},
				{"a", "b", "e"},
			},
			// avg=0.5 > 0.4 → warn
			wantLevel: "warn",
			wantMin:   0.49,
			wantMax:   0.51,
		},
		{
			name:   "hard threshold triggered",
			window: 3,
			warnAt: 0.4,
			hardAt: 0.9,
			entries: [][]string{
				{"a", "b", "c"},
				{"a", "b", "c"},
				{"a", "b", "c"},
			},
			// identical → avg=1.0 > 0.9 → hard (checked first)
			wantLevel: "hard",
			wantMin:   1.0,
			wantMax:   1.0,
		},
		{
			name:   "hard takes priority over warn",
			window: 2,
			warnAt: 0.5,
			hardAt: 0.8,
			entries: [][]string{
				{"a", "b", "c"},
				{"a", "b", "c"},
			},
			// identical → 1.0 > hardAt → hard (not warn)
			wantLevel: "hard",
			wantMin:   1.0,
			wantMax:   1.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := NewSimilarityDetector(tc.window, tc.warnAt, tc.hardAt)
			for _, entry := range tc.entries {
				d.Push(entry)
			}
			level, score := d.Check()
			if level != tc.wantLevel {
				t.Errorf("level = %q, want %q", level, tc.wantLevel)
			}
			if score < tc.wantMin || score > tc.wantMax {
				t.Errorf("score = %f, want [%f, %f]", score, tc.wantMin, tc.wantMax)
			}
		})
	}
}
