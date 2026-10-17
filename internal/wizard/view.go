package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/quanarch/quanarch/internal/quanarch"
)

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#5B3FD1", Dark: "#A78BFA"}
	colorDim    = lipgloss.AdaptiveColor{Light: "#8A8698", Dark: "#76727F"}
	colorError  = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF6B6B"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#1E8E5A", Dark: "#4ADE80"}

	kindColors = map[quanarch.Kind]lipgloss.AdaptiveColor{
		quanarch.Quanar: {Light: "#B7791F", Dark: "#F2B84B"},
		quanarch.Quan:   {Light: "#7C3AED", Dark: "#B794F6"},
		quanarch.Qua:    {Light: "#0F8A7E", Dark: "#4FD1C5"},
		quanarch.Qmock:  {Light: "#C2417F", Dark: "#F687B3"},
	}

	styleBrand  = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#1D1B26"))
	styleDim    = lipgloss.NewStyle().Foreground(colorDim)
	styleBold   = lipgloss.NewStyle().Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleError  = lipgloss.NewStyle().Foreground(colorError)
	styleOK     = lipgloss.NewStyle().Foreground(colorOK).Bold(true)
	styleBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorDim).Padding(0, 2).MarginLeft(2)
)

func kindStyle(k quanarch.Kind) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(kindColors[k]).Bold(true)
}

func question(s step) string {
	switch s {
	case stepKind:
		return "What do you want to create?"
	case stepLanguage:
		return "Which language?"
	case stepDir:
		return "Where should it go?"
	case stepName:
		return "What is its name?"
	case stepConfirm:
		return "Create it?"
	}
	return ""
}

// answer is the chosen value of a completed step, as shown above the current one.
func (m Model) answer(s step) string {
	switch s {
	case stepKind:
		return kindStyle(m.answers.Kind).Render(string(m.answers.Kind))
	case stepLanguage:
		return styleAccent.Render(m.answers.Language.Name)
	case stepDir:
		return styleAccent.Render(m.answers.Dir)
	case stepName:
		return styleAccent.Render(m.answers.Name)
	}
	return ""
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n " + styleBrand.Render("quanarch") + " " + styleDim.Render(m.version) + "\n\n")

	if m.answers.Kind != "" && !m.has(stepKind) {
		fmt.Fprintf(&b, " %s %s %s\n", styleOK.Render("✔"), styleBold.Render(question(stepKind)), m.answer(stepKind))
	}
	if m.answers.Name != "" && !m.has(stepName) {
		fmt.Fprintf(&b, " %s %s %s\n", styleOK.Render("✔"), styleBold.Render(question(stepName)), m.answer(stepName))
	}
	for i := 0; i < m.at; i++ {
		s := m.steps[i]
		fmt.Fprintf(&b, " %s %s %s\n", styleOK.Render("✔"), styleBold.Render(question(s)), m.answer(s))
	}

	if m.cancelled {
		b.WriteString("\n " + styleError.Render("✖") + " Cancelled. Nothing was created.\n\n")
		return b.String()
	}
	if m.done {
		b.WriteString(m.summary())
		b.WriteString("\n " + styleOK.Render("✔") + " " + styleBold.Render("Ready to create "+string(m.answers.Kind)+" "+m.answers.Name) + "\n")
		b.WriteString("   " + styleDim.Render("Preview only: nothing was written to disk.") + "\n\n")
		return b.String()
	}

	s := m.current()
	fmt.Fprintf(&b, " %s %s\n", styleAccent.Render("?"), styleBold.Render(question(s)))
	b.WriteString(m.body(s))

	if m.err != "" {
		b.WriteString("\n   " + styleError.Render(m.err) + "\n")
	}
	b.WriteString("\n " + styleDim.Render(m.help(s)) + "\n")
	return b.String()
}

func (m Model) has(s step) bool {
	for _, x := range m.steps {
		if x == s {
			return true
		}
	}
	return false
}

func (m Model) body(s step) string {
	var b strings.Builder
	switch s {
	case stepKind:
		width := 0
		for _, k := range quanarch.Kinds {
			width = max(width, len(k))
		}
		for i, k := range quanarch.Kinds {
			pointer, name := "  ", string(k)
			if i == m.kindCursor {
				pointer = styleAccent.Render("❯ ")
			}
			fmt.Fprintf(&b, "   %s%s %s  %s\n", pointer, kindStyle(k).Render("●"),
				kindStyle(k).Render(fmt.Sprintf("%-*s", width, name)), styleDim.Render(k.Summary()))
		}

	case stepLanguage:
		width := 0
		for _, l := range Languages {
			width = max(width, lipgloss.Width(l.Name))
		}
		for i, l := range Languages {
			pointer := "  "
			if i == m.langCursor {
				pointer = styleAccent.Render("❯ ")
			}
			padded := l.Name + strings.Repeat(" ", width-lipgloss.Width(l.Name))
			var line string
			switch {
			case !l.Available:
				line = styleDim.Render(padded + "  soon")
			case i == m.langCursor:
				line = styleAccent.Render(padded)
			default:
				line = padded
			}
			fmt.Fprintf(&b, "   %s%s\n", pointer, line)
		}

	case stepDir:
		b.WriteString("   " + styleAccent.Render("› ") + m.dirInput.View() + "\n")
		if dir, err := m.resolveDir(m.dirInput.Value()); err == nil {
			b.WriteString("   " + styleDim.Render(dir) + "\n")
		}

	case stepName:
		b.WriteString("   " + styleAccent.Render("› ") + m.nameInput.View() + "\n")
		b.WriteString("   " + styleDim.Render("lowercase letters, digits and dashes") + "\n")

	case stepConfirm:
		b.WriteString(m.summary())
		yes, no := " Yes ", " No "
		if m.confirmYes {
			yes = styleBrand.Render(yes)
			no = styleDim.Render(no)
		} else {
			yes = styleDim.Render(yes)
			no = styleBrand.Render(no)
		}
		b.WriteString("\n   " + yes + "  " + no + "\n")
	}
	return b.String()
}

func (m Model) summary() string {
	rows := []string{
		styleDim.Render("Kind      ") + kindStyle(m.answers.Kind).Render(string(m.answers.Kind)),
		styleDim.Render("Name      ") + styleBold.Render(m.answers.Name),
		styleDim.Render("Language  ") + m.answers.Language.Name,
		styleDim.Render("Location  ") + m.answers.Target(),
	}
	return "\n" + styleBox.Render(strings.Join(rows, "\n")) + "\n"
}

func (m Model) help(s step) string {
	back := "esc back"
	if m.at == 0 {
		back = "esc cancel"
	}
	switch s {
	case stepKind, stepLanguage:
		return "↑/↓ move · enter select · " + back + " · ctrl+c quit"
	case stepConfirm:
		return "←/→ choose · enter confirm · " + back + " · ctrl+c quit"
	}
	return "enter continue · " + back + " · ctrl+c quit"
}
