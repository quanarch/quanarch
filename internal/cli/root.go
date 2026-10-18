// Package cli implements the quanarch command-line interface.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/banner"
	"github.com/quanarch/quanarch/internal/version"
	"github.com/quanarch/quanarch/internal/wizard"
)

// Execute runs the CLI with the process arguments and returns the exit code.
func Execute() int {
	return run(programName(os.Args[0]), os.Args[1:], os.Stdout, os.Stderr)
}

func run(name string, args []string, stdout, stderr io.Writer) int {
	root := newRootCmd(name)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	return 0
}

func init() {
	// Keep commands in declaration order so kinds read quan, quanar, qua.
	cobra.EnableCommandSorting = false
}

func newRootCmd(name string) *cobra.Command {
	root := &cobra.Command{
		Use:   name,
		Short: "Create Quanarch projects, quanars, quans, quas and qmocks",
		Long: banner.Text(version.Label()) + `
Run qu alone to be asked step by step, or say everything in one command:

  qu new <kind> [name] [flags]

Every command explains itself: "qu new --help", then "qu new <kind> --help".
Learn more at https://quanarch.org`,
		Version:       version.String(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare `qu` opens the interactive wizard.
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !wizard.IsInteractive() {
				return cmd.Help()
			}
			return runNew(cmd, wizard.Preset{})
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(
		newNewCmd(),
		newCheckCmd(),
		newVersionCmd(),
	)
	return root
}

// programName returns the name the binary was invoked as, so help text
// reads "qu" when run through the short alias.
func programName(arg0 string) string {
	name := strings.TrimSuffix(filepath.Base(arg0), ".exe")
	if name == "qu" {
		return name
	}
	return "quanarch"
}
