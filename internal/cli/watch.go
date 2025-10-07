package cli

import (
	"fmt"

	"github.com/interpretive-systems/diffium/internal/gitx"
	"github.com/interpretive-systems/diffium/internal/tui"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Open the TUI and watch for changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := mustGetStringFlag(cmd.Root(), "repo")
			root, err := gitx.RepoRoot(repoPath)
			if err != nil {
				return fmt.Errorf("not a git repo: %w", err)
			}
			return tui.Run(root)
		},
	}
	return cmd
}
