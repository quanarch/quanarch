package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
	want := Answers{Kind: quanarch.Quan, Language: Languages[0], Dir: filepath.Join(cwd, "apps"), Name: "billing-report"}
	if m.Answers() != want {
		t.Errorf("answers = %+v, want %+v", m.Answers(), want)
	}
	if _, err := os.Stat(filepath.Join(cwd, "apps")); !os.IsNotExist(err) {
		t.Error("the wizard wrote to disk")
	}
}

func TestPresetSkipsSteps(t *testing.T) {
	m := New(Preset{Kind: quanarch.Qmock, Name: "dev"}, t.TempDir(), "v0")
	if len(m.steps) != 3 || m.current() != stepLanguage {
		t.Fatalf("steps = %v", m.steps)
	}
	m = press(t, m, "enter", "enter") // language, current folder
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
	m = press(t, m, "enter", "enter", "left", "enter")
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
