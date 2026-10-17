package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/quanarch"
)

// errNoTemplates is returned until the template engine lands.
var errNoTemplates = errors.New("templates are not available yet")

type newOptions struct {
	stack string
	dir   string
}

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <kind> <name>",
		Short: "Create a new quan, quanar or qua from a stack template",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	for _, kind := range quanarch.Kinds {
		cmd.AddCommand(newKindCmd(kind))
	}
	return cmd
}

func newKindCmd(kind quanarch.Kind) *cobra.Command {
	var opts newOptions
	cmd := &cobra.Command{
		Use:   string(kind) + " <name>",
		Short: fmt.Sprintf("Create a new %s, %s", kind, kind.Summary()),
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := quanarch.ValidateName(args[0]); err != nil {
				return err
			}
			return errNoTemplates
		},
	}
	cmd.Flags().StringVarP(&opts.stack, "stack", "s", "", "stack to use, for example react-ts or react-ts@1")
	cmd.Flags().StringVarP(&opts.dir, "dir", "d", "", "directory to create the project in (default ./<name>)")
	return cmd
}
