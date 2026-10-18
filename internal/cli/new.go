package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	createpkg "github.com/quanarch/quanarch/internal/create"
	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/source"
	"github.com/quanarch/quanarch/internal/stack"
	"github.com/quanarch/quanarch/internal/version"
	"github.com/quanarch/quanarch/internal/wizard"
)

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new [kind] [name]",
		Short: "Create a quanar, quan, qua or qmock",
		Long: "Create a quanar, quan, qua or qmock.\n\n" +
			"Whatever is not given on the command line is asked for interactively.",
		Example: "  qu new\n  qu new quan\n  qu new quan billing",
		Args:    cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var preset wizard.Preset
			if len(args) > 0 {
				kind, err := quanarch.ParseKind(args[0])
				if err != nil {
					return err
				}
				preset.Kind = kind
			}
			if len(args) > 1 {
				if err := quanarch.ValidateName(args[1]); err != nil {
					return err
				}
				preset.Name = args[1]
			}
			return runWizard(cmd, preset)
		},
	}
}

// runWizard asks for what preset leaves out. Nothing is created yet: the
// wizard only collects and confirms the answers.
func runWizard(cmd *cobra.Command, preset wizard.Preset) error {
	if !wizard.IsInteractive() {
		var missing []string
		if preset.Kind == "" {
			missing = append(missing, "kind")
		}
		missing = append(missing, "language", "location")
		if preset.Name == "" {
			missing = append(missing, "name")
		}
		return fmt.Errorf("%s must be chosen in an interactive terminal", strings.Join(missing, ", "))
	}

	answers, err := wizard.Run(preset, "v"+strings.TrimPrefix(version.Version, "v"), cmd.OutOrStdout())
	if errors.Is(err, wizard.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}
	return create(cmd, answers)
}

func create(cmd *cobra.Command, a wizard.Answers) error {
	out := cmd.OutOrStdout()
	if a.Kind == wizard.Examples {
		fmt.Fprintf(out, " The %s example is not available yet.\n\n", a.Example.Name)
		return nil
	}
	st, err := resolveStack(cmd, a.Language)
	if err != nil {
		return err
	}
	if a.Kind != quanarch.Project {
		return createSingle(cmd, a, st)
	}
	p := createpkg.Project{Dir: a.Dir, Name: a.Name, Qua: a.Qua, Quanar: a.Quanar, Quan: a.Quan}
	if err := createpkg.NewProject(p, st, true, out); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n Done. Run it in two terminals:\n\n")
	fmt.Fprintf(out, "   cd %s && npm run watch\n", filepath.Join(a.Name, "quans", a.Quan))
	fmt.Fprintf(out, "   cd %s && npm run dev\n\n", filepath.Join(a.Name, "quanar"))
	fmt.Fprintf(out, " Then open http://localhost:5173/%s\n\n", a.Quan)
	return nil
}

func createSingle(cmd *cobra.Command, a wizard.Answers, st createpkg.Stack) error {
	out := cmd.OutOrStdout()
	target := a.Target()
	res, err := createpkg.NewSingle(createpkg.Single{Kind: a.Kind, Name: a.Name, Target: target, Qua: a.QuaSource}, st, true, out)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n Done.\n\n")
	switch a.Kind {
	case quanarch.Quan:
		fmt.Fprintf(out, "   cd %s && npm run watch    # serves the quan on port %d\n\n", target, res.Port)
		for _, file := range res.Registered {
			fmt.Fprintf(out, " Added to %s\n", file)
		}
		for _, host := range res.Wired {
			fmt.Fprintf(out, " Plugged its qua into %s\n", host)
		}
		if len(res.Registered) == 0 {
			fmt.Fprintf(out, " Add it to your quanar's quan list:\n   { name: '%s', url: 'http://localhost:%d/%s.js' }\n", a.Name, res.Port, a.Name)
		}
	case quanarch.Quanar, quanarch.Qmock:
		fmt.Fprintf(out, "   cd %s && npm run dev    # http://localhost:%d\n", target, res.Port)
	case quanarch.Qua:
		fmt.Fprintf(out, " Built into %s. Link it from a quanar and its quans.\n", filepath.Join(target, "dist"))
	}
	for _, note := range res.Notes {
		fmt.Fprintf(out, "\n ! %s\n", note)
	}
	if needsQua := a.Kind != quanarch.Qua; needsQua && a.QuaSource.From == createpkg.NoQua {
		fmt.Fprintf(out, "\n It has no qua yet: it cannot talk to a quanar until one is added. See its README.\n")
	}
	fmt.Fprintln(out)
	return nil
}

// resolveStack finds the templates for a language. QU_STACK_DIR points at a
// stack on disk instead, which is how stacks are developed.
func resolveStack(cmd *cobra.Command, lang wizard.Language) (createpkg.Stack, error) {
	dir := os.Getenv("QU_STACK_DIR")
	if dir == "" {
		spec, err := source.ParseSpec(lang.ID)
		if err != nil {
			return createpkg.Stack{}, err
		}
		cacheDir, err := source.DefaultCacheDir()
		if err != nil {
			return createpkg.Stack{}, err
		}
		fetched, err := (&source.Fetcher{CacheDir: cacheDir}).Fetch(cmd.Context(), spec)
		if err != nil {
			return createpkg.Stack{}, err
		}
		dir = fetched.Dir
	}
	m, err := stack.LoadManifest(dir)
	if err != nil {
		return createpkg.Stack{}, err
	}
	return createpkg.Stack{Dir: dir, Manifest: m}, nil
}
