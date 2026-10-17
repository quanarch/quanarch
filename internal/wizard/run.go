package wizard

import (
	"errors"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

// ErrCancelled is returned when the user leaves the wizard without confirming.
var ErrCancelled = errors.New("cancelled")

// IsInteractive reports whether the wizard can talk to a terminal.
func IsInteractive() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

// Run shows the wizard and returns the confirmed answers.
func Run(preset Preset, version string, out io.Writer) (Answers, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Answers{}, err
	}
	final, err := tea.NewProgram(New(preset, cwd, version), tea.WithOutput(out)).Run()
	if err != nil {
		return Answers{}, err
	}
	m := final.(Model)
	if !m.Done() {
		return Answers{}, ErrCancelled
	}
	return m.Answers(), nil
}
