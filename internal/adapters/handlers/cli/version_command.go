package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCommand prints the binary version and build date.
// Example: idx version
func (runner CommandRunner) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version and build information",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "idx %s (built %s)\n", runner.buildInfo.Version, runner.buildInfo.BuildDate)
		},
	}
}
