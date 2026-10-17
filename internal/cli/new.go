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
	if a.Kind != quanarch.Project {
		fmt.Fprintf(out, " Creating a single %s is not connected yet; choose project to create files.\n\n", a.Kind)
		return nil
	}

	st, err := resolveStack(cmd, a.Language)
	if err != nil {
		return err
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
