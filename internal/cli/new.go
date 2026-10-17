package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/quanarch"
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

	_, err := wizard.Run(preset, "v"+strings.TrimPrefix(version.Version, "v"), cmd.OutOrStdout())
	if errors.Is(err, wizard.ErrCancelled) {
		return nil
	}
	return err
}
