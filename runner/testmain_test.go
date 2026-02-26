package runner_test

import (
	"os"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

func TestMain(m *testing.M) {
	if testutil.RunMockClaude() {
		return
	}
	os.Exit(m.Run())
}
