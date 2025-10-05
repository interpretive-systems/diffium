// package cli
//
// import (
//     "fmt"
//
//     "github.com/interpretive-systems/diffium/internal/gitx"
//     "github.com/interpretive-systems/diffium/internal/tui"
//     "github.com/spf13/cobra"
// )
//
// func newWatchCmd() *cobra.Command {
//     cmd := &cobra.Command{
//         Use:   "watch",
//         Short: "Open the TUI and watch for changes",
//         RunE: func(cmd *cobra.Command, args []string) error {
//             repoPath := mustGetStringFlag(cmd.Root(), "repo")
//             root, err := gitx.RepoRoot(repoPath)
//             if err != nil {
//                 return fmt.Errorf("not a git repo: %w", err)
//             }
//             return tui.Run(root)
//         },
//     }
//     return cmd
// }
//


package cli

import (
	"fmt"

	"github.com/interpretive-systems/diffium/internal/gitx"
	"github.com/interpretive-systems/diffium/internal/tui"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	var theme string // 1. Define a variable to hold the flag value

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Open the TUI and watch for changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := mustGetStringFlag(cmd.Root(), "repo")
			root, err := gitx.RepoRoot(repoPath)
			if err != nil {
				return fmt.Errorf("not a git repo: %w", err)
			}

			// 3. Pass the flag value to the updated tui.Run function
			return tui.Run(root, theme)
		},
	}

	// 2. Add the --theme flag to the command
	cmd.Flags().StringVar(
		&theme,
		"theme",
		"dark", // Set "dark" as the default theme
		"The color theme to use: 'dark' or 'light'.",
	)

	return cmd
}
