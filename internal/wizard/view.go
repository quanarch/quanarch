package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/quanarch/quanarch/internal/banner"
	"github.com/quanarch/quanarch/internal/create"
	"github.com/quanarch/quanarch/internal/quanarch"
)

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#5B3FD1", Dark: "#A78BFA"}
	colorDim    = lipgloss.AdaptiveColor{Light: "#8A8698", Dark: "#76727F"}
	colorError  = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF6B6B"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#1E8E5A", Dark: "#4ADE80"}

	kindColors = map[quanarch.Kind]lipgloss.AdaptiveColor{
		quanarch.Project: {Light: "#2563EB", Dark: "#7AA2F7"},
		quanarch.Quanar:  {Light: "#B7791F", Dark: "#F2B84B"},
		quanarch.Quan:    {Light: "#7C3AED", Dark: "#B794F6"},
		quanarch.Qua:     {Light: "#0F8A7E", Dark: "#4FD1C5"},
		quanarch.Qmock:   {Light: "#C2417F", Dark: "#F687B3"},
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

func (m Model) question(s step) string {
	project := m.answers.Kind == quanarch.Project
	switch s {
	case stepKind:
		return "What do you want to create?"
	case stepLanguage:
		return "Which language?"
	case stepDir:
		return "Where should it go?"
	case stepName:
		if project {
			return "Project name?"
		}
		return "What is its name?"
	case stepQuaName:
		return "Qua name?"
	case stepQuanarName:
		return "Quanar name?"
	case stepQuanName:
		return "Quan name?"
	case stepQua:
		return "Which qua should it use?"
	case stepNewQua:
		return "New qua name?"
	case stepQuaSpec:
		return "Which published qua?"
	case stepExample:
		return "Which example?"
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
	case stepQuaName:
		return kindStyle(quanarch.Qua).Render(m.answers.Qua)
	case stepQuanarName:
		return kindStyle(quanarch.Quanar).Render(m.answers.Quanar)
	case stepQuanName:
		return kindStyle(quanarch.Quan).Render(m.answers.Quan)
	case stepQua, stepNewQua, stepQuaSpec:
		return kindStyle(quanarch.Qua).Render(m.quaSummary())
	case stepExample:
		return styleAccent.Render(m.answers.Example.Name)
	}
	return ""
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	for _, row := range banner.Logo() {
		b.WriteString(" " + styleBold.Render(row) + "\n")
	}
	b.WriteString(" " + styleDim.Render(m.version) + "\n " + styleDim.Render(banner.Definition) + "\n\n")

	// Answers given on the command line come first, as if already asked.
	for _, s := range m.givenSteps() {
		fmt.Fprintf(&b, " %s %s %s\n", styleOK.Render("✔"), styleBold.Render(m.question(s)), m.answer(s))
	}
	for i := 0; i < m.at; i++ {
		s := m.steps[i]
		fmt.Fprintf(&b, " %s %s %s\n", styleOK.Render("✔"), styleBold.Render(m.question(s)), m.answer(s))
	}

	if m.cancelled {
		b.WriteString("\n " + styleError.Render("✖") + " Cancelled. Nothing was created.\n\n")
		return b.String()
	}
	if m.done {
		b.WriteString(m.summary() + "\n")
		return b.String()
	}

	s := m.current()
	fmt.Fprintf(&b, " %s %s\n", styleAccent.Render("?"), styleBold.Render(m.question(s)))
	b.WriteString(m.body(s))

	if m.err != "" {
		b.WriteString("\n   " + styleError.Render(m.err) + "\n")
	}
	if m.showHelp {
		b.WriteString("\n" + styleBox.Render(stepHelp(s)) + "\n")
	}
	b.WriteString("\n " + styleDim.Render(m.help(s)+" · ? help") + "\n")
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
		for _, k := range menu {
			width = max(width, len(k))
		}
		for i, k := range menu {
			pointer, name := "  ", string(k)
			if i == m.kindCursor {
				pointer = styleAccent.Render("❯ ")
			}
			fmt.Fprintf(&b, "   %s%s %s  %s\n", pointer, kindStyle(k).Render("●"),
				kindStyle(k).Render(fmt.Sprintf("%-*s", width, name)), styleDim.Render(menuSummary(k)))
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

	case stepExample:
		list := m.examples()
		if len(list) == 0 {
			b.WriteString("   " + styleDim.Render("No examples for "+m.answers.Language.Name+" yet.") + "\n")
			break
		}
		width := 0
		for _, ex := range list {
			width = max(width, lipgloss.Width(ex.Name))
		}
		for i, ex := range list {
			pointer, name := "  ", ex.Name+strings.Repeat(" ", width-lipgloss.Width(ex.Name))
			summary := ex.Summary
			if !ex.Available {
				summary += "  · soon"
			}
			switch {
			case !ex.Available:
				name = styleDim.Render(name)
			case i == m.exCursor:
				name = styleAccent.Render(name)
			}
			if i == m.exCursor {
				pointer = styleAccent.Render("❯ ")
			}
			fmt.Fprintf(&b, "   %s%s  %s\n", pointer, name, styleDim.Render(summary))
		}

	case stepQua:
		width := 0
		for _, o := range m.quaOptions {
			width = max(width, lipgloss.Width(o.label))
		}
		for i, o := range m.quaOptions {
			pointer, label := "  ", o.label+strings.Repeat(" ", width-lipgloss.Width(o.label))
			if i == m.quaCursor {
				pointer = styleAccent.Render("❯ ")
				label = styleAccent.Render(label)
			}
			fmt.Fprintf(&b, "   %s%s  %s\n", pointer, label, styleDim.Render(o.hint))
		}

	case stepName, stepQuaName, stepQuanarName, stepQuanName, stepNewQua, stepQuaSpec:
		b.WriteString("   " + styleAccent.Render("› ") + m.names[s].View() + "\n")
		b.WriteString("   " + styleDim.Render(m.nameHint(s)) + "\n")

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

func (m Model) nameHint(s step) string {
	switch s {
	case stepQuaName:
		return "the contract, created in packages/"
	case stepQuanarName:
		return "the host, created in quanar/"
	case stepQuanName:
		return "the first quan, created in quans/"
	case stepNewQua:
		return "created in the project's packages/ and linked"
	case stepQuaSpec:
		return "name@version or name@url, for example acme-qua@^1.0.0"
	}
	return "lowercase letters, digits and dashes"
}

func (m Model) summary() string {
	label := func(s string) string { return styleDim.Render(fmt.Sprintf("%-10s", s)) }
	rows := []string{
		label("Kind") + kindStyle(m.answers.Kind).Render(string(m.answers.Kind)),
		label("Name") + styleBold.Render(m.answers.Name),
	}
	if m.answers.Kind == quanarch.Project {
		rows = append(rows,
			label("Qua")+kindStyle(quanarch.Qua).Render(m.answers.Qua)+styleDim.Render("     packages/"+m.answers.Qua),
			label("Quanar")+kindStyle(quanarch.Quanar).Render(m.answers.Quanar)+styleDim.Render("  quanar/"),
			label("Quan")+kindStyle(quanarch.Quan).Render(m.answers.Quan)+styleDim.Render("    quans/"+m.answers.Quan),
		)
	}
	if needsQua(m.answers.Kind) {
		rows = append(rows, label("Qua")+kindStyle(quanarch.Qua).Render(m.quaSummary()))
	}
	rows = append(rows,
		label("Language")+m.answers.Language.Name,
		label("Location")+m.answers.Target(),
	)
	return "\n" + styleBox.Render(strings.Join(rows, "\n")) + "\n"
}

func (m Model) help(s step) string {
	back := "esc back"
	if m.at == 0 {
		back = "esc cancel"
	}
	switch s {
	case stepKind, stepLanguage, stepExample, stepQua:
		return "↑/↓ move · enter select · " + back + " · ctrl+c quit"
	case stepConfirm:
		return "←/→ choose · enter confirm · " + back + " · ctrl+c quit"
	}
	return "enter continue · " + back + " · ctrl+c quit"
}

// quaSummary describes the chosen qua in a few words.
func (m Model) quaSummary() string {
	q := m.answers.QuaSource
	switch q.From {
	case create.LocalQua:
		return q.Name + " (linked from packages/)"
	case create.NewQua:
		if q.Name == "" {
			return "a new qua"
		}
		return q.Name + " (new, in packages/)"
	case create.PublishedQua:
		if q.Name == "" {
			return "a published qua"
		}
		return q.Name + "@" + q.Spec
	}
	return "none for now"
}

func menuSummary(k quanarch.Kind) string {
	if k == Examples {
		return "complete applications to learn from or start with"
	}
	return k.Summary()
}

// givenSteps lists the questions the command line already answered.
func (m Model) givenSteps() []step {
	p := m.preset
	var given []step
	add := func(s step, ok bool) {
		if ok {
			given = append(given, s)
		}
	}
	add(stepKind, p.Kind != "")
	add(stepLanguage, p.Language != nil)
	add(stepExample, p.Example != "" && m.answers.Kind == Examples)
	add(stepDir, p.Dir != "")
	add(stepName, p.Name != "")
	add(stepQuaName, p.QuaName != "")
	add(stepQuanarName, p.QuanarName != "")
	add(stepQuanName, p.QuanName != "")
	add(stepQua, p.Qua != nil)
	return given
}

// stepHelp explains a question and the flag that answers it from the command line.
func stepHelp(s step) string {
	switch s {
	case stepKind:
		return "What to create. project makes a qua, a quanar and a quan together.\nCommand line: qu new <kind>, for example qu new quan"
	case stepLanguage:
		return "The language and framework of the templates.\nCommand line: --lang react-ts"
	case stepExample:
		return "Which complete application to create.\nCommand line: --example advanced"
	case stepDir:
		return "The folder to create it in. A project root puts a quan in quans/,\na qua in packages/ and a qmock in qmock/.\nCommand line: --dir ."
	case stepName:
		return "Lowercase letters, digits and dashes. It names the folder and the package.\nCommand line: qu new <kind> <name>"
	case stepQuaName, stepQuanarName, stepQuanName:
		return "Each part of a project becomes its own package, so the names must differ.\nCommand line: --qua, --quanar, --quan"
	case stepQua:
		return "The contract this one talks through. Inside a project: one of its quas or a\nnew one; anywhere: a published qua, or none for now.\nCommand line: --qua <name|path|name@version|none> or --new-qua <name>"
	case stepNewQua:
		return "The new qua is created in the project's packages/ and linked.\nCommand line: --new-qua <name>"
	case stepQuaSpec:
		return "An npm package with a version range, or a URL.\nCommand line: --qua acme-qua@^1.0.0"
	case stepConfirm:
		return "Nothing is written until you confirm. When a command answers every\nquestion, qu creates at once without asking."
	}
	return ""
}
