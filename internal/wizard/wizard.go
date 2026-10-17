// Package wizard asks, step by step, for whatever was not given on the command line.
package wizard

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/quanarch/quanarch/internal/quanarch"
)

// Language is a language and framework a project can be created in.
type Language struct {
	ID        string
	Name      string
	Available bool
}

// Languages lists every language in the order they are offered.
var Languages = []Language{
	{ID: "react-ts", Name: "React · TypeScript", Available: true},
	{ID: "vue-ts", Name: "Vue · TypeScript"},
	{ID: "angular", Name: "Angular · TypeScript"},
	{ID: "svelte-ts", Name: "Svelte · TypeScript"},
	{ID: "react-native", Name: "React Native · TypeScript"},
	{ID: "node-ts", Name: "Node.js · TypeScript"},
	{ID: "python", Name: "Python"},
	{ID: "go", Name: "Go"},
	{ID: "rust", Name: "Rust"},
	{ID: "java", Name: "Java · Spring"},
	{ID: "dotnet", Name: ".NET · C#"},
	{ID: "php", Name: "PHP · Laravel"},
	{ID: "flutter", Name: "Flutter · Dart"},
}

// Preset holds what was already given on the command line; empty fields are asked for.
type Preset struct {
	Kind quanarch.Kind
	Name string
}

// Answers is the outcome of a completed wizard.
type Answers struct {
	Kind     quanarch.Kind
	Language Language
	// Dir is the absolute directory the project is created in.
	Dir  string
	Name string
	// For a project: the names of its qua, quanar and quan.
	Qua    string
	Quanar string
	Quan   string
}

// Target is the absolute path of the project to create.
func (a Answers) Target() string {
	return filepath.Join(a.Dir, a.Name)
}

type step int

const (
	stepKind step = iota
	stepLanguage
	stepDir
	stepName
	stepQuaName
	stepQuanarName
	stepQuanName
	stepConfirm
)

// Model is the wizard's state. It is a Bubble Tea model.
type Model struct {
	steps []step
	at    int

	answers    Answers
	kindCursor int
	langCursor int
	dirInput   textinput.Model
	names      map[step]*textinput.Model
	confirmYes bool
	err        string

	cwd        string
	version    string
	presetKind bool
	presetName bool
	done       bool
	cancelled  bool
}

// New returns a wizard that asks for everything preset leaves out.
func New(preset Preset, cwd, version string) Model {
	m := Model{cwd: cwd, version: version, confirmYes: true, names: map[step]*textinput.Model{}}
	m.answers.Kind = preset.Kind
	m.answers.Name = preset.Name
	m.presetKind = preset.Kind != ""
	m.presetName = preset.Name != ""

	m.dirInput = textinput.New()
	m.dirInput.Placeholder = "."
	m.dirInput.Prompt = ""
	m.dirInput.CharLimit = 4096

	for _, s := range []step{stepName, stepQuaName, stepQuanarName, stepQuanName} {
		in := textinput.New()
		in.Prompt = ""
		in.CharLimit = 64
		m.names[s] = &in
	}
	m.names[stepName].Placeholder = "billing-report"
	m.names[stepQuaName].Placeholder = "acme-qua"
	m.names[stepQuanarName].Placeholder = "acme-quanar"
	m.names[stepQuanName].Placeholder = "billing"

	m.steps = m.buildSteps()
	return m
}

// buildSteps lists the steps for the chosen kind; a project asks for more names.
func (m Model) buildSteps() []step {
	var steps []step
	if !m.presetKind {
		steps = append(steps, stepKind)
	}
	steps = append(steps, stepLanguage, stepDir)
	if !m.presetName {
		steps = append(steps, stepName)
	}
	if m.answers.Kind == quanarch.Project {
		steps = append(steps, stepQuaName, stepQuanarName, stepQuanName)
	}
	return append(steps, stepConfirm)
}

// Done reports whether the user confirmed; Answers is then complete.
func (m Model) Done() bool { return m.done }

// Answers returns what the user chose.
func (m Model) Answers() Answers { return m.answers }

func (m Model) current() step { return m.steps[m.at] }

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.updateInput(msg)
	}

	switch key.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		return m.back()
	}

	switch m.current() {
	case stepKind:
		return m.updateKind(key)
	case stepLanguage:
		return m.updateLanguage(key)
	case stepDir:
		return m.updateDir(key)
	case stepName, stepQuaName, stepQuanarName, stepQuanName:
		return m.updateName(key)
	case stepConfirm:
		return m.updateConfirm(key)
	}
	return m, nil
}

func (m Model) back() (tea.Model, tea.Cmd) {
	m.err = ""
	if m.at == 0 {
		m.cancelled = true
		return m, tea.Quit
	}
	m.at--
	return m, m.focus()
}

func (m Model) next() (tea.Model, tea.Cmd) {
	m.err = ""
	m.at++
	return m, m.focus()
}

// focus gives keyboard focus to the text input of the current step, if any.
func (m *Model) focus() tea.Cmd {
	m.dirInput.Blur()
	for _, in := range m.names {
		in.Blur()
	}
	if m.current() == stepDir {
		return m.dirInput.Focus()
	}
	if in, ok := m.names[m.current()]; ok {
		return in.Focus()
	}
	return nil
}

func (m Model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.current() == stepDir {
		m.dirInput, cmd = m.dirInput.Update(msg)
	} else if in, ok := m.names[m.current()]; ok {
		*in, cmd = in.Update(msg)
	}
	return m, cmd
}

func moveCursor(cursor, n int, key string) int {
	switch key {
	case "up", "k", "shift+tab":
		return (cursor - 1 + n) % n
	case "down", "j", "tab":
		return (cursor + 1) % n
	}
	return cursor
}

func (m Model) updateKind(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "enter" {
		m.answers.Kind = quanarch.Kinds[m.kindCursor]
		m.steps = m.buildSteps()
		return m.next()
	}
	m.kindCursor = moveCursor(m.kindCursor, len(quanarch.Kinds), key.String())
	return m, nil
}

func (m Model) updateLanguage(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "enter" {
		lang := Languages[m.langCursor]
		if !lang.Available {
			m.err = lang.Name + " is coming soon."
			return m, nil
		}
		m.answers.Language = lang
		return m.next()
	}
	m.err = ""
	m.langCursor = moveCursor(m.langCursor, len(Languages), key.String())
	return m, nil
}

func (m Model) updateDir(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() != "enter" {
		m.err = ""
		return m.updateInput(key)
	}
	dir, err := m.resolveDir(m.dirInput.Value())
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.answers.Dir = dir
	if m.answers.Name != "" {
		if err := checkTarget(m.answers.Target()); err != nil {
			m.err = err.Error()
			return m, nil
		}
	}
	return m.next()
}

func (m Model) updateName(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() != "enter" {
		next, cmd := m.updateInput(key)
		nm := next.(Model)
		nm.err = nm.nameError()
		return nm, cmd
	}
	name := strings.TrimSpace(m.names[m.current()].Value())
	if err := quanarch.ValidateName(name); err != nil {
		m.err = err.Error()
		return m, nil
	}
	if err := m.checkUnique(m.current(), name); err != nil {
		m.err = err.Error()
		return m, nil
	}
	switch m.current() {
	case stepName:
		if err := checkTarget(filepath.Join(m.answers.Dir, name)); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.answers.Name = name
	case stepQuaName:
		m.answers.Qua = name
	case stepQuanarName:
		m.answers.Quanar = name
	case stepQuanName:
		m.answers.Quan = name
	}
	return m.next()
}

// checkUnique keeps the qua, quanar and quan of a project apart: each becomes a package.
func (m Model) checkUnique(s step, name string) error {
	taken := map[step]string{stepQuaName: m.answers.Qua, stepQuanarName: m.answers.Quanar, stepQuanName: m.answers.Quan}
	for other, used := range taken {
		if other != s && used == name && s != stepName {
			return fmt.Errorf("%q is already used in this project", name)
		}
	}
	return nil
}

// nameError validates the name as it is typed; an empty field is not an error yet.
func (m Model) nameError() string {
	name := m.names[m.current()].Value()
	if name == "" {
		return ""
	}
	if err := quanarch.ValidateName(name); err != nil {
		return err.Error()
	}
	return ""
}

func (m Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "left", "right", "h", "l", "tab", "shift+tab", "up", "down":
		m.confirmYes = !m.confirmYes
	case "y":
		m.confirmYes = true
	case "n":
		m.confirmYes = false
	case "enter":
		if m.confirmYes {
			m.done = true
		} else {
			m.cancelled = true
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) resolveDir(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		input = "."
	}
	if strings.HasPrefix(input, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			input = filepath.Join(home, strings.TrimPrefix(input, "~"))
		}
	}
	if !filepath.IsAbs(input) {
		input = filepath.Join(m.cwd, input)
	}
	dir := filepath.Clean(input)
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, nil // created later
	case err != nil:
		return "", err
	case !info.IsDir():
		return "", fmt.Errorf("%s is a file, not a folder", dir)
	}
	return dir, nil
}

// checkTarget refuses a project folder that already has something in it.
func checkTarget(target string) error {
	entries, err := os.ReadDir(target)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty", target)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		if info, statErr := os.Stat(target); statErr == nil && !info.IsDir() {
			return fmt.Errorf("%s already exists and is a file", target)
		}
	}
	return nil
}
