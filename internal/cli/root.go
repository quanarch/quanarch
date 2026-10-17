// Package cli implements the quanarch command-line interface.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/version"
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
		Use:           name,
		Short:         "Create and manage Quanarch quans, quanars and quas",
		Long:          "Quanarch is an architecture style built on three concepts: quan, quanar and qua.\nLearn more at https://quanarch.org",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(
		newNewCmd(),
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
