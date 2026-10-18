package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/quanarch/quanarch/internal/create"
	"github.com/quanarch/quanarch/internal/quanarch"
)

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		case "ctrl+u":
			msg = tea.KeyMsg{Type: tea.KeyCtrlU}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestFullFlow(t *testing.T) {
	cwd := t.TempDir()
	m := New(Preset{}, cwd, "v0.0.0")

	m = press(t, m, "down", "down", "enter") // quan
	if m.answers.Kind != quanarch.Quan || m.current() != stepLanguage {
		t.Fatalf("after kind: %+v at %d", m.answers, m.current())
	}
	m = press(t, m, "enter")          // React · TypeScript
	m = press(t, m, "apps", "enter")  // location
	m = press(t, m, "billing-report") // name
	m = press(t, m, "enter")
	m = press(t, m, "down", "enter") // qua: none for now (outside a project: published, none)
	if m.current() != stepConfirm {
		t.Fatalf("expected confirm, at %d err=%q", m.current(), m.err)
	}
	if !strings.Contains(m.View(), filepath.Join(cwd, "apps", "billing-report")) {
		t.Errorf("summary does not show the target:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if !m.Done() {
		t.Fatal("not done after confirming")
	}
	want := Answers{Kind: quanarch.Quan, Language: Languages[0], Dir: filepath.Join(cwd, "apps"), Name: "billing-report", QuaSource: create.QuaSource{From: create.NoQua}}
	if m.Answers() != want {
		t.Errorf("answers = %+v, want %+v", m.Answers(), want)
	}
	if _, err := os.Stat(filepath.Join(cwd, "apps")); !os.IsNotExist(err) {
		t.Error("the wizard wrote to disk")
	}
}

func TestPresetSkipsSteps(t *testing.T) {
	m := New(Preset{Kind: quanarch.Qmock, Name: "dev"}, t.TempDir(), "v0")
	if len(m.steps) != 4 || m.current() != stepLanguage {
		t.Fatalf("steps = %v", m.steps)
	}
	m = press(t, m, "enter", "enter") // language, current folder
	m = press(t, m, "down", "enter")  // no qua
	if m.current() != stepConfirm || m.answers.Name != "dev" {
		t.Fatalf("at %d, answers %+v", m.current(), m.answers)
	}
}

func TestEscGoesBackAndCancelsAtStart(t *testing.T) {
	m := New(Preset{}, t.TempDir(), "v0")
	m = press(t, m, "down", "enter") // quanar
	if m.current() != stepLanguage {
		t.Fatal("did not advance")
	}
	m = press(t, m, "esc")
	if m.current() != stepKind {
		t.Fatal("esc did not go back")
	}
	m = press(t, m, "esc")
	if !m.cancelled || m.Done() {
		t.Fatal("esc on the first step did not cancel")
	}
}

func TestUnavailableLanguageIsRefused(t *testing.T) {
	m := New(Preset{Kind: quanarch.Quan}, t.TempDir(), "v0")
	m = press(t, m, "down", "enter")
	if m.current() != stepLanguage || !strings.Contains(m.err, "coming soon") {
		t.Fatalf("at %d err=%q", m.current(), m.err)
	}
}

func TestNameIsValidatedAsTyped(t *testing.T) {
	m := New(Preset{Kind: quanarch.Qua}, t.TempDir(), "v0")
	m = press(t, m, "enter", "enter") // language, folder
	m = press(t, m, "my qua")
	if !strings.Contains(m.err, "kebab-case") {
		t.Fatalf("err = %q", m.err)
	}
	m = press(t, m, "enter")
	if m.current() != stepName {
		t.Fatal("an invalid name was accepted")
	}
}

func TestExistingFolderIsRefused(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "billing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "billing", "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(Preset{Kind: quanarch.Quan}, cwd, "v0")
	m = press(t, m, "enter", "enter", "billing", "enter")
	if m.current() != stepName || !strings.Contains(m.err, "not empty") {
		t.Fatalf("at %d err=%q", m.current(), m.err)
	}
}

func TestDeclining(t *testing.T) {
	m := New(Preset{Kind: quanarch.Quan, Name: "x"}, t.TempDir(), "v0")
	m = press(t, m, "enter", "enter", "down", "enter", "left", "enter")
	if m.Done() || !m.cancelled {
		t.Fatal("declining did not cancel")
	}
	if !strings.Contains(m.View(), "Nothing was created") {
		t.Errorf("view:\n%s", m.View())
	}
}

func TestProjectAsksForEveryName(t *testing.T) {
	cwd := t.TempDir()
	m := New(Preset{}, cwd, "v0")
	m = press(t, m, "enter") // project
	if m.answers.Kind != quanarch.Project {
		t.Fatalf("kind = %s", m.answers.Kind)
	}
	m = press(t, m, "enter", "enter")    // language, current folder
	m = press(t, m, "acme", "enter")     // project
	m = press(t, m, "acme-qua", "enter") // qua
	m = press(t, m, "acme-qua", "enter") // quanar: same name is refused
	if m.current() != stepQuanarName || !strings.Contains(m.err, "already used") {
		t.Fatalf("at %d err=%q", m.current(), m.err)
	}
	m = press(t, m, "esc", "enter")                   // back to qua, keep it
	m = press(t, m, "ctrl+u", "acme-quanar", "enter") // clear the refused name first
	m = press(t, m, "billing", "enter")
	if m.current() != stepConfirm {
		t.Fatalf("at %d err=%q", m.current(), m.err)
	}
	view := m.View()
	for _, want := range []string{"packages/acme-qua", "quanar/", "quans/billing", filepath.Join(cwd, "acme")} {
		if !strings.Contains(view, want) {
			t.Errorf("summary misses %q:\n%s", want, view)
		}
	}
	m = press(t, m, "enter")
	want := Answers{Kind: quanarch.Project, Language: Languages[0], Dir: cwd, Name: "acme", Qua: "acme-qua", Quanar: "acme-quanar", Quan: "billing"}
	if !m.Done() || m.Answers() != want {
		t.Errorf("answers = %+v", m.Answers())
	}
}

// newProjectDir lays out a project root with one qua, as qu creates it.
func newProjectDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range map[string]string{
		"conf.qu":                   "project acme\n",
		"packages/acme-qua/conf.qu": "qua acme-qua\nversion 1\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestQuanInsideProjectUsesItsQua(t *testing.T) {
	root := newProjectDir(t)
	m := New(Preset{Kind: quanarch.Quan, Name: "invoices"}, root, "v0")
	m = press(t, m, "enter", "enter") // language, project root
	labels := []string{}
	for _, o := range m.quaOptions {
		labels = append(labels, o.label)
	}
	if strings.Join(labels, "|") != "acme-qua|Create a new qua|Use a published qua|No qua for now" {
		t.Fatalf("options = %v", labels)
	}
	m = press(t, m, "enter", "enter") // acme-qua, confirm
	a := m.Answers()
	if !m.Done() || a.Target() != filepath.Join(root, "quans", "invoices") {
		t.Fatalf("target = %s", a.Target())
	}
	if a.QuaSource.From != create.LocalQua || a.QuaSource.Dir != filepath.Join(root, "packages", "acme-qua") {
		t.Errorf("qua = %+v", a.QuaSource)
	}
}

func TestCreatingANewQuaAsksItsName(t *testing.T) {
	root := newProjectDir(t)
	m := New(Preset{Kind: quanarch.Quan, Name: "invoices"}, root, "v0")
	m = press(t, m, "enter", "enter", "down", "enter") // create a new qua
	if m.current() != stepNewQua {
		t.Fatalf("at %d", m.current())
	}
	m = press(t, m, "acme-qua", "enter")
	if !strings.Contains(m.err, "not empty") {
		t.Fatalf("an existing qua folder was accepted: err=%q", m.err)
	}
	m = press(t, m, "ctrl+u", "billing-qua", "enter", "enter")
	if !m.Done() || m.Answers().QuaSource != (create.QuaSource{From: create.NewQua, Name: "billing-qua"}) {
		t.Fatalf("qua = %+v", m.Answers().QuaSource)
	}
}

func TestPublishedQua(t *testing.T) {
	m := New(Preset{Kind: quanarch.Quanar, Name: "host"}, t.TempDir(), "v0")
	m = press(t, m, "enter", "enter", "enter") // language, folder, published
	m = press(t, m, "Bad Name", "enter")
	if !strings.Contains(m.err, "kebab-case") {
		t.Fatalf("err = %q", m.err)
	}
	m = press(t, m, "ctrl+u", "acme-qua@^1.2.0", "enter", "enter")
	if !m.Done() || m.Answers().QuaSource != (create.QuaSource{From: create.PublishedQua, Name: "acme-qua", Spec: "^1.2.0"}) {
		t.Fatalf("qua = %+v", m.Answers().QuaSource)
	}
}

func TestExamplesAsksForLanguageThenExample(t *testing.T) {
	m := New(Preset{}, t.TempDir(), "v0")
	m = press(t, m, "up", "enter") // examples is the last entry
	if m.answers.Kind != Examples || m.current() != stepLanguage {
		t.Fatalf("kind=%s at %d", m.answers.Kind, m.current())
	}
	m = press(t, m, "enter") // React · TypeScript
	if m.current() != stepExample || !strings.Contains(m.View(), "Advanced") {
		t.Fatalf("at %d:\n%s", m.current(), m.View())
	}
	m = press(t, m, "enter", "enter", "shop", "enter") // advanced, current folder, name
	if m.current() != stepConfirm || m.answers.Example.ID != "advanced" {
		t.Fatalf("at %d err=%q example=%+v", m.current(), m.err, m.answers.Example)
	}
	if !strings.Contains(m.View(), "shop") {
		t.Errorf("summary:\n%s", m.View())
	}
}

func TestUnreleasedLanguageWithAStackDir(t *testing.T) {
	var lang Language
	for _, l := range Languages {
		if !l.Available {
			lang = l
			break
		}
	}
	a := Answers{Kind: quanarch.Qua, Name: "acme-qua", Language: lang}
	if err := Validate(a); err == nil || !strings.Contains(err.Error(), "coming soon") {
		t.Fatalf("an unreleased language was accepted: %v", err)
	}
	t.Setenv("QU_STACK_DIR", t.TempDir())
	if err := Validate(a); err != nil {
		t.Errorf("with QU_STACK_DIR: %v", err)
	}
}
