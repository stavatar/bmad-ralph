package plan

import "fmt"

// MaxInputBytes is the threshold above which CheckSize warns about input size.
const MaxInputBytes = 100_000

// CheckSize returns a warning if the total size of all inputs exceeds MaxInputBytes.
func CheckSize(inputs []PlanInput) (warn bool, msg string) {
	var total int
	for _, inp := range inputs {
		total += len(inp.Content)
	}
	if total > MaxInputBytes {
		return true, fmt.Sprintf("суммарный размер входных документов: %dKB (лимит ~100KB). Рекомендуется разбить через 'bmad shard-doc'", total/1024)
	}
	return false, ""
}
