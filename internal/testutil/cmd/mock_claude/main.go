package main

import (
	"fmt"
	"os"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

func main() {
	if !testutil.RunMockClaude() {
		fmt.Fprintf(os.Stderr, "mock_claude: MOCK_CLAUDE_SCENARIO not set\n")
		os.Exit(1)
	}
}
