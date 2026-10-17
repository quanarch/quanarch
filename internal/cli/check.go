package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/conf"
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [dir]",
		Short: "Validate the " + conf.FileName + " file of a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			p, err := conf.Load(dir)
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("no %s found in %s", conf.FileName, dir)
			}
			if err != nil {
				return err
			}
			return printProject(cmd.OutOrStdout(), p)
		},
	}
}

func printProject(w io.Writer, p *conf.Project) error {
	ew := &errWriter{w: w}
	ew.printf("%s %s", p.Kind, p.Name)
	if p.Version != "" {
		ew.printf(" %s", p.Version)
	}
	ew.printf("\n")
	if p.Stack != nil {
		ew.printf("  stack  %s\n", p.Stack)
	}
	for _, q := range p.Quas {
		ew.printf("  qua    %s\n", q)
	}
	for _, z := range p.Zones {
		ew.printf("  zone   %s\n", z.Name)
		for _, q := range z.Quas {
			ew.printf("           qua %s\n", q)
		}
	}
	return ew.err
}

// errWriter keeps the first write error so a sequence of prints can be checked once.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err == nil {
		_, e.err = fmt.Fprintf(e.w, format, args...)
	}
}
