package create

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quanarch/quanarch/internal/stack"
)

// writeStack creates a minimal stack whose templates record the data they received.
func writeStack(t *testing.T) Stack {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"stack.qu":                     "stack demo\nversion 1.0.0\ntemplate project t/project\ntemplate qua t/qua\ntemplate quanar t/quanar\ntemplate quan t/quan\n",
		"t/project/info.qtpl":          "[[ .Kind ]] [[ .Name ]] qua=[[ (index .Quas 0).Path ]] quan=[[ (index .Quans 0).Name ]]:[[ (index .Quans 0).Port ]]\n",
		"t/project/packages/README.md": "packages\n",
		"t/qua/info.qtpl":              "[[ .Kind ]] [[ .Name ]] project=[[ .Project ]]\n",
		"t/quanar/info.qtpl":           "[[ .Kind ]] [[ .Name ]] qua=[[ (index .Quas 0).Path ]] quan=[[ (index .Quans 0).Name ]]\n",
		"t/quan/info.qtpl":             "[[ .Kind ]] [[ .Name ]] qua=[[ (index .Quas 0).Path ]] port=[[ .Port ]]\n",
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := stack.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	return Stack{Dir: dir, Manifest: m}
}

func TestNewProjectLayout(t *testing.T) {
	st := writeStack(t)
	parent := t.TempDir()
	p := Project{Dir: parent, Name: "acme", Qua: "acme-qua", Quanar: "acme-quanar", Quan: "billing"}
	if err := NewProject(p, st, false, io.Discard); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"acme/info":                   "project acme qua=packages/acme-qua quan=billing:4001\n",
		"acme/packages/README.md":     "packages\n",
		"acme/packages/acme-qua/info": "qua acme-qua project=acme\n",
		"acme/quanar/info":            "quanar acme-quanar qua=../packages/acme-qua quan=billing\n",
		"acme/quans/billing/info":     "quan billing qua=../../packages/acme-qua port=4001\n",
	}
	for rel, content := range want {
		got, err := os.ReadFile(filepath.Join(parent, filepath.FromSlash(rel)))
		if err != nil || string(got) != content {
			t.Errorf("%s = %q, %v; want %q", rel, got, err, content)
		}
	}
}

func TestNewProjectRefusesExistingFolder(t *testing.T) {
	st := writeStack(t)
	parent := t.TempDir()
	if err := os.MkdirAll(filepath.Join(parent, "acme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "acme", "keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := NewProject(Project{Dir: parent, Name: "acme", Qua: "q", Quanar: "r", Quan: "n"}, st, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewProjectNeedsEveryTemplate(t *testing.T) {
	st := writeStack(t)
	delete(st.Manifest.Templates, "quan")
	err := NewProject(Project{Dir: t.TempDir(), Name: "acme", Qua: "q", Quanar: "r", Quan: "n"}, st, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no template for quan") {
		t.Fatalf("err = %v", err)
	}
}

// TestStackProject creates a project from a real stack, installs it and builds
// every part. It runs only when QU_E2E_STACK_DIR points at a stack, as in a
// stack repository's CI.
func TestStackProject(t *testing.T) {
	dir := os.Getenv("QU_E2E_STACK_DIR")
	if dir == "" {
		t.Skip("QU_E2E_STACK_DIR is not set")
	}
	m, err := stack.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	p := Project{Dir: parent, Name: "acme", Qua: "acme-qua", Quanar: "acme-quanar", Quan: "billing"}
	if err := NewProject(p, Stack{Dir: dir, Manifest: m}, true, testWriter{t}); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(parent, "acme")
	for _, step := range []struct{ dir, script string }{
		{"quans/billing", "typecheck"},
		{"quans/billing", "build"},
		{"quanar", "build"},
	} {
		cmd := exec.Command("npm", "run", step.script)
		cmd.Dir = filepath.Join(root, filepath.FromSlash(step.dir))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("npm run %s in %s: %v\n%s", step.script, step.dir, err, out)
		}
	}

	bundle, err := os.ReadFile(filepath.Join(root, "quans", "billing", "dist", "billing.js"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bundle), "react.transitional.element") {
		t.Error("the quan bundles its own React")
	}
	if !strings.Contains(string(bundle), "window.BillingQuan") {
		t.Error("the quan does not publish its manifest on window.BillingQuan")
	}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
