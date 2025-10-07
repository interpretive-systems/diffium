package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() error {
	root := &cobra.Command{
		Use:   "diffium",
		Short: "Diff-first TUI for git changes",
		Long:  "Diffium: Explore and review git diffs in a side-by-side TUI.",
	}

	root.PersistentFlags().StringP("repo", "r", ".", "Path to repository root (default: current dir)")

	// Add subcommands
	root.AddCommand(newWatchCmd())

	if err := root.Execute(); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}

func mustGetStringFlag(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "flag error:", err)
		os.Exit(2)
	}
	return v
}
