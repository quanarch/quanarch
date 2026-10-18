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

	"github.com/quanarch/quanarch/internal/create"
	"github.com/quanarch/quanarch/internal/project"
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

// Examples is the menu entry for complete example applications. It is not a
// kind of Quanarch building block, only something qu can set up.
const Examples quanarch.Kind = "examples"

// menu lists the entries of the first question.
var menu = append(append([]quanarch.Kind{}, quanarch.Kinds...), Examples)

// Example is a complete application in one language.
type Example struct {
	ID        string
	Name      string
	Summary   string
	Available bool
}

// ExamplesByLanguage lists the examples of each language, by language ID.
var ExamplesByLanguage = map[string][]Example{
	"react-ts": {
		{ID: "advanced", Name: "Advanced", Summary: "server, artifacts store, quanar, qua and four quans loaded four ways", Available: true},
	},
}

// Preset holds what was already given on the command line; empty fields are asked for.
type Preset struct {
	Kind     quanarch.Kind
	Name     string
	Language *Language
	// Dir is absolute when given.
	Dir string
	// Qua is the qua of a quanar, quan or qmock.
	Qua *create.QuaSource
	// The names of a project's qua, quanar and quan.
	QuaName    string
	QuanarName string
	QuanName   string
	// Example is the ID of an example.
	Example string
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
	// For a quanar, quan or qmock: where its qua comes from.
	QuaSource create.QuaSource
	// For examples: the chosen example.
	Example Example
}

// Target is the absolute path of the folder to create. Inside a project root a
// quan goes in quans/, a qua in packages/ and so on.
func (a Answers) Target() string {
	if a.Kind == quanarch.Project || a.Kind == "" {
		return filepath.Join(a.Dir, a.Name)
	}
	return project.Place(a.Kind, a.Dir, a.Name)
}

// needsQua reports whether the kind is wired to a qua.
func needsQua(k quanarch.Kind) bool {
	return k == quanarch.Quanar || k == quanarch.Quan || k == quanarch.Qmock
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
	stepQua
	stepNewQua
	stepQuaSpec
	stepExample
	stepConfirm
)

// quaOption is one answer to "which qua?".
type quaOption struct {
	label  string
	hint   string
	source create.QuaSource
}

// Model is the wizard's state. It is a Bubble Tea model.
type Model struct {
	steps []step
	at    int

	answers    Answers
	kindCursor int
	langCursor int
	dirInput   textinput.Model
	names      map[step]*textinput.Model
	quaOptions []quaOption
	quaCursor  int
	exCursor   int
	confirmYes bool
	showHelp   bool
	err        string

	cwd       string
	version   string
	preset    Preset
	done      bool
	cancelled bool
}

// New returns a wizard that asks for everything preset leaves out.
func New(preset Preset, cwd, version string) Model {
	m := Model{cwd: cwd, version: version, confirmYes: true, names: map[step]*textinput.Model{}, preset: preset}
	m.answers.Kind = preset.Kind
	m.answers.Name = preset.Name
	m.answers.Dir = preset.Dir
	m.answers.Qua = preset.QuaName
	m.answers.Quanar = preset.QuanarName
	m.answers.Quan = preset.QuanName
	if preset.Language != nil {
		m.answers.Language = *preset.Language
		for _, ex := range ExamplesByLanguage[preset.Language.ID] {
			if ex.ID == preset.Example {
				m.answers.Example = ex
			}
		}
	}
	if preset.Qua != nil {
		m.answers.QuaSource = *preset.Qua
	}

	m.dirInput = textinput.New()
	m.dirInput.Placeholder = "."
	m.dirInput.Prompt = ""
	m.dirInput.CharLimit = 4096

	for _, s := range []step{stepName, stepQuaName, stepQuanarName, stepQuanName, stepNewQua, stepQuaSpec} {
		in := textinput.New()
		in.Prompt = ""
		in.CharLimit = 64
		m.names[s] = &in
	}
	m.names[stepName].Placeholder = "billing-report"
	m.names[stepQuaName].Placeholder = "acme-qua"
	m.names[stepQuanarName].Placeholder = "acme-quanar"
	m.names[stepQuanName].Placeholder = "billing"
	m.names[stepNewQua].Placeholder = "acme-qua"
	m.names[stepQuaSpec].Placeholder = "acme-qua@^1.0.0"
	m.names[stepQuaSpec].CharLimit = 2048

	m.steps = m.buildSteps()
	return m
}

// buildSteps lists the steps for the chosen kind; a project asks for more names.
func (m Model) buildSteps() []step {
	p := m.preset
	var steps []step
	add := func(s step, given bool) {
		if !given {
			steps = append(steps, s)
		}
	}
	add(stepKind, p.Kind != "")
	add(stepLanguage, p.Language != nil)
	if m.answers.Kind == Examples {
		add(stepExample, p.Example != "")
		add(stepDir, p.Dir != "")
		add(stepName, p.Name != "")
		return append(steps, stepConfirm)
	}
	add(stepDir, p.Dir != "")
	add(stepName, p.Name != "")
	if m.answers.Kind == quanarch.Project {
		add(stepQuaName, p.QuaName != "")
		add(stepQuanarName, p.QuanarName != "")
		add(stepQuanName, p.QuanName != "")
	}
	if needsQua(m.answers.Kind) && p.Qua == nil {
		steps = append(steps, stepQua)
		switch m.answers.QuaSource.From {
		case create.NewQua:
			steps = append(steps, stepNewQua)
		case create.PublishedQua:
			steps = append(steps, stepQuaSpec)
		}
	}
	return append(steps, stepConfirm)
}

// Complete reports whether the command line answered everything: nothing to ask.
func (m Model) Complete() bool {
	return m.answers.Kind != "" && len(m.steps) == 1
}

// Missing names what is still to be answered, as the flags that would answer it.
func (m Model) Missing() []string {
	flags := map[step]string{
		stepKind:       "the kind (qu new <kind>)",
		stepLanguage:   "--lang",
		stepExample:    "--example",
		stepDir:        "--dir",
		stepName:       "the name (qu new <kind> <name>)",
		stepQuaName:    "--qua",
		stepQuanarName: "--quanar",
		stepQuanName:   "--quan",
		stepQua:        "--qua or --new-qua",
	}
	var missing []string
	for _, s := range m.steps {
		if f, ok := flags[s]; ok {
			missing = append(missing, f)
		}
	}
	return missing
}

// Validate checks answers given on the command line, as the wizard would have.
func Validate(a Answers) error {
	if !a.Language.Available {
		return fmt.Errorf("%s is coming soon", a.Language.Name)
	}
	if a.Kind == Examples && !a.Example.Available {
		return fmt.Errorf("example %q is not available for %s", a.Example.ID, a.Language.Name)
	}
	if a.Kind == quanarch.Project {
		names := map[string]string{}
		for _, n := range []struct{ role, name string }{{"qua", a.Qua}, {"quanar", a.Quanar}, {"quan", a.Quan}} {
			if err := quanarch.ValidateName(n.name); err != nil {
				return fmt.Errorf("--%s: %w", n.role, err)
			}
			if other, taken := names[n.name]; taken {
				return fmt.Errorf("--%s and --%s are both %q; each becomes its own package", other, n.role, n.name)
			}
			names[n.name] = n.role
		}
	}
	if a.QuaSource.From == create.NewQua {
		root, ok := project.FindRoot(a.Dir)
		if !ok {
			return fmt.Errorf("--new-qua needs a project: run it inside one, or point --dir at one")
		}
		if err := checkTarget(filepath.Join(root, "packages", a.QuaSource.Name)); err != nil {
			return err
		}
	}
	return checkTarget(a.Target())
}

// buildQuaOptions offers the quas of the surrounding project, a new one, a published one or none.
func (m Model) buildQuaOptions() []quaOption {
	var opts []quaOption
	root, inProject := project.FindRoot(m.answers.Dir)
	if inProject {
		for _, q := range project.Quas(root) {
			rel, _ := filepath.Rel(root, q.Dir)
			hint := filepath.ToSlash(rel)
			if q.Version != "" {
				hint += " · v" + q.Version
			}
			opts = append(opts, quaOption{label: q.Name, hint: hint, source: create.QuaSource{From: create.LocalQua, Name: q.Name, Dir: q.Dir}})
		}
		opts = append(opts, quaOption{label: "Create a new qua", hint: "in packages/", source: create.QuaSource{From: create.NewQua}})
	}
	opts = append(opts,
		quaOption{label: "Use a published qua", hint: "an npm package or a URL", source: create.QuaSource{From: create.PublishedQua}},
		quaOption{label: "No qua for now", hint: "its README will say one is needed", source: create.QuaSource{From: create.NoQua}},
	)
	return opts
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
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	}

	switch m.current() {
	case stepKind:
		return m.updateKind(key)
	case stepLanguage:
		return m.updateLanguage(key)
	case stepDir:
		return m.updateDir(key)
	case stepName, stepQuaName, stepQuanarName, stepQuanName, stepNewQua:
		return m.updateName(key)
	case stepQua:
		return m.updateQua(key)
	case stepQuaSpec:
		return m.updateQuaSpec(key)
	case stepExample:
		return m.updateExample(key)
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
	if m.current() == stepQua {
		m.quaOptions = m.buildQuaOptions()
		if m.quaCursor >= len(m.quaOptions) {
			m.quaCursor = 0
		}
	}
	return m, m.focus()
}

// examples returns the examples of the chosen language.
func (m Model) examples() []Example {
	return ExamplesByLanguage[m.answers.Language.ID]
}

func (m Model) updateExample(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.examples()
	if len(list) == 0 {
		return m, nil
	}
	if key.String() == "enter" {
		ex := list[m.exCursor]
		if !ex.Available {
			m.err = "The " + strings.ToLower(ex.Name) + " example is coming soon."
			return m, nil
		}
		m.answers.Example = ex
		return m.next()
	}
	m.err = ""
	m.exCursor = moveCursor(m.exCursor, len(list), key.String())
	return m, nil
}

func (m Model) updateQua(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "enter" {
		m.answers.QuaSource = m.quaOptions[m.quaCursor].source
		m.steps = m.buildSteps()
		return m.next()
	}
	m.quaCursor = moveCursor(m.quaCursor, len(m.quaOptions), key.String())
	return m, nil
}

func (m Model) updateQuaSpec(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() != "enter" {
		m.err = ""
		return m.updateInput(key)
	}
	name, spec, err := ParseQuaSpec(m.names[stepQuaSpec].Value())
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.answers.QuaSource = create.QuaSource{From: create.PublishedQua, Name: name, Spec: spec}
	return m.next()
}

// ParseQuaSpec reads "name", "name@^1.0.0" or "name@https://…".
func ParseQuaSpec(s string) (name, spec string, err error) {
	s = strings.TrimSpace(s)
	name, spec, found := strings.Cut(s, "@")
	if err := quanarch.ValidateName(name); err != nil {
		return "", "", fmt.Errorf("qua %v", err)
	}
	if !found || spec == "" {
		spec = "latest"
	}
	return name, spec, nil
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
		m.answers.Kind = menu[m.kindCursor]
		m.steps = m.buildSteps()
		return m.next()
	}
	m.kindCursor = moveCursor(m.kindCursor, len(menu), key.String())
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
		a := m.answers
		a.Name = name
		if err := checkTarget(a.Target()); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.answers.Name = name
	case stepNewQua:
		if name == m.answers.Name {
			m.err = fmt.Sprintf("%q is already the %s's name", name, m.answers.Kind)
			return m, nil
		}
		root, _ := project.FindRoot(m.answers.Dir)
		if err := checkTarget(filepath.Join(root, "packages", name)); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.answers.QuaSource = create.QuaSource{From: create.NewQua, Name: name}
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
	return ResolveDir(m.cwd, input)
}

// ResolveDir turns a typed location into an absolute folder: empty means cwd,
// ~ means home, relative paths start at cwd. A missing folder is fine.
func ResolveDir(cwd, input string) (string, error) {
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
		input = filepath.Join(cwd, input)
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
