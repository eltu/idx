package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (runner CommandRunner) newReadCommand() *cobra.Command {
	var fromLine, toLine int

	cmd := &cobra.Command{
		Use:   "read <path>",
		Short: "Print the contents of a project file to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if runner.readCommand == nil {
				return fmt.Errorf("read command not configured")
			}
			return runner.readCommand.RunWithOptions(args[0], fromLine, toLine)
		},
	}

	cmd.Flags().IntVar(&fromLine, "from", 0, "First line to print, 1-based (0 = start of file)")
	cmd.Flags().IntVar(&toLine, "to", 0, "Last line to print, 1-based (0 = end of file)")
	return cmd
}
