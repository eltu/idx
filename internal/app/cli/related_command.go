package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"idx/internal/features/related"
)

const relatedLongDescription = `Find files most likely relevant to the given file.

Uses two signals combined in a weighted sum:
  - Co-read affinity (weight 0.7): files read within ±2h of the target
  - Term co-occurrence (weight 0.3): files sharing significant vocabulary

Signals:
  co-read       Files frequently read together in the same session window
  term-overlap  Files sharing significant BM25 vocabulary with the target
  both          Both signals detected

When the read log is empty, falls back to term-overlap only.

Examples:
  idx related internal/features/search/service.go
  idx related internal/features/search/service.go --limit 5
  idx related internal/features/search/service.go --format json`

func (runner CommandRunner) newRelatedCommand() *cobra.Command {
	var size int
	var format string

	cmd := &cobra.Command{
		Use:   "related <file>",
		Short: "Find files related to the given file",
		Long:  relatedLongDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if runner.relatedCommand == nil {
				return fmt.Errorf("related command is not available")
			}
			opts := related.Options{Format: format, Size: size}
			return runner.relatedCommand.Run(args[0], opts)
		},
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveDefault
		},
	}

	cmd.Flags().IntVarP(&size, "limit", "n", 10, "Maximum number of related files to return")
	cmd.Flags().StringVar(&format, "format", related.OutputText, "Output format: text|json")
	return cmd
}
