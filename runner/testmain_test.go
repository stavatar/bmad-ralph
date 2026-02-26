package runner_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

func TestMain(m *testing.M) {
	// Self-reexec: exit with given code and empty stdout (for exit error + parse failure tests).
	// Checked before RunMockClaude per self-reexec dispatch pattern.
	if code := os.Getenv("MOCK_EXIT_EMPTY"); code != "" {
		n, _ := strconv.Atoi(code)
		os.Exit(n)
	}
	if testutil.RunMockClaude() {
		return
	}
	os.Exit(m.Run())
}
