package bridge

import (
	"context"
	"fmt"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// Run converts story files to sprint-tasks.md.
// Epic 2 (Story 2.3) implements the full bridge logic.
func Run(ctx context.Context, cfg *config.Config, storyFiles []string) error {
	return fmt.Errorf("bridge: not implemented")
}
