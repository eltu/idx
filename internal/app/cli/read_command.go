package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const readLongDescription = `Print the raw contents of a project file to stdout.
Each read is logged to boost the file's BM25 popularity score in future searches.

Examples:
  idx read internal/app/cli/search_command.go
  idx open README.md --start 10 --end 50
  idx read -s 100 -e 120 internal/features/search/service.go`

func (runner CommandRunner) newReadCommand() *cobra.Command {
	var fromLine, toLine int

	cmd := &cobra.Command{
		Use:     "read <path>",
		Aliases: []string{"open", "cat"},
		Short:   "Print the contents of a project file to stdout",
		Long:    readLongDescription,
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if runner.readCommand == nil {
				return fmt.Errorf("read command not configured")
			}
			return runner.readCommand.RunWithOptions(args[0], fromLine, toLine)
		},
	}

	cmd.Flags().IntVarP(&fromLine, "start", "s", 0, "First line to print, 1-based (0 = start of file)")
	cmd.Flags().IntVarP(&toLine, "end", "e", 0, "Last line to print, 1-based (0 = end of file)")

	// Deprecated aliases kept for backward compatibility.
	cmd.Flags().IntVar(&fromLine, "from", 0, "")
	cmd.Flags().IntVar(&toLine, "to", 0, "")
	_ = cmd.Flags().MarkHidden("from")
	_ = cmd.Flags().MarkHidden("to")
	_ = cmd.Flags().MarkDeprecated("from", "use --start instead")
	_ = cmd.Flags().MarkDeprecated("to", "use --end instead")

	return cmd
}
