package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", cmd.Root().Name(), version.String())
			return err
		},
	}
}
