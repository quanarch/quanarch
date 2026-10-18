package create

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/render"
	"github.com/quanarch/quanarch/internal/stack"
)

// hookStack is a stack that says, in its own words, how a quanar takes in a quan and a qua.
func hookStack(t *testing.T, hooks string) Stack {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"stack.qu": "stack demo\n" +
			"template project t/project\ntemplate qua t/qua\ntemplate quanar t/quanar {\n\tport 3000\n}\n" +
			"template quan t/quan {\n\tport 5001\n}\n" + hooks,
		"t/project/conf.qu.qtpl": "project [[ .Name ]]\n",
		"t/qua/conf.qu.qtpl":     "qua [[ .Name ]]\nversion 1\n",
		"t/quanar/conf.qu.qtpl":  "quanar [[ .Name ]]\nport [[ .Port ]]\n[[ range .Quas ]]qua [[ .Name ]]@[[ .Version ]]\n[[ end ]]",
		"t/quanar/src/list.txt":  "quans:\n  // qu:quans\n",
		"t/quanar/src/core.txt":  "cores:\n// qu:cores\n",
		"t/quan/conf.qu.qtpl":    "quan [[ .Name ]]\nport [[ .Port ]]\n[[ range .Quas ]]qua [[ .Name ]]@[[ .Version ]]\n[[ end ]]",
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

const demoHooks = `on quan insert src/list.txt "// qu:quans" "[[ .Name ]] :[[ .Port ]] [[ range .Quas ]][[ .Name ]][[ end ]]"
on qua insert src/core.txt "// qu:cores" {
	line "core [[ .Name ]] {"
	line "  from [[ .Dependency ]]"
	line "  path [[ (index .Quas 0).Path ]]"
	line "}"
}
`

func newDemoProject(t *testing.T, st Stack) string {
	t.Helper()
	parent := t.TempDir()
	p := Project{Dir: parent, Name: "acme", Qua: "acme-qua", Quanar: "acme-quanar", Quan: "billing"}
	if err := NewProject(p, st, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, "acme")
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNewQuanIsTakenInTheWayTheStackSays(t *testing.T) {
	st := hookStack(t, demoHooks)
	root := newDemoProject(t, st)
	quanar := filepath.Join(root, "quanar")

	if got := read(t, filepath.Join(root, "quans", "billing", "conf.qu")); !strings.Contains(got, "port 5001") {
		t.Errorf("the first quan does not get the stack's port: %q", got)
	}
	if got := read(t, filepath.Join(quanar, "conf.qu")); !strings.Contains(got, "port 3000") {
		t.Errorf("the quanar does not get the stack's port: %q", got)
	}

	// A quan whose qua the quanar already has: only the quan is added.
	res, err := NewSingle(Single{Kind: quanarch.Quan, Name: "stock", Target: filepath.Join(root, "quans", "stock"),
		Qua: QuaSource{From: LocalQua, Name: "acme-qua", Dir: filepath.Join(root, "packages", "acme-qua")}}, st, false, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if res.Port != 5002 || len(res.Registered) != 1 || len(res.Wired) != 0 || len(res.Notes) != 0 {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, filepath.Join(quanar, "src", "list.txt")); got != "quans:\n  stock :5002 acme-qua\n  // qu:quans\n" {
		t.Errorf("list.txt = %q", got)
	}
	if got := read(t, filepath.Join(quanar, "src", "core.txt")); got != "cores:\n// qu:cores\n" {
		t.Errorf("a qua the quanar has was plugged again: %q", got)
	}

	// A quan that brings a new qua: the qua is plugged in and recorded in the quanar's conf.qu.
	res, err = NewSingle(Single{Kind: quanarch.Quan, Name: "shipping", Target: filepath.Join(root, "quans", "shipping"),
		Qua: QuaSource{From: NewQua, Name: "ship-qua"}}, st, false, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if res.Port != 5003 || len(res.Wired) != 1 || len(res.Registered) != 1 {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, filepath.Join(quanar, "src", "core.txt")); got != "cores:\ncore ship-qua {\n  from file:../packages/ship-qua\n  path ../packages/ship-qua\n}\n// qu:cores\n" {
		t.Errorf("core.txt = %q", got)
	}
	if got := read(t, filepath.Join(quanar, "conf.qu")); !strings.HasSuffix(got, "qua acme-qua@1\nqua ship-qua@1\n") {
		t.Errorf("conf.qu = %q", got)
	}
}

func TestHooksAreNotAppliedTwice(t *testing.T) {
	st := hookStack(t, demoHooks)
	root := newDemoProject(t, st)
	quanar := filepath.Join(root, "quanar")
	data := renderData("stock", 5002, "acme-qua")

	for i, want := range []bool{true, false} {
		changed, err := applyHooks(st, quanarch.Quan, quanar, data, false, io.Discard)
		if err != nil || changed != want {
			t.Errorf("round %d: changed=%v err=%v, want %v", i, changed, err, want)
		}
	}
	if got := strings.Count(read(t, filepath.Join(quanar, "src", "list.txt")), "stock"); got != 1 {
		t.Errorf("stock is listed %d times", got)
	}
}

func TestMissingMarkerChangesNothing(t *testing.T) {
	st := hookStack(t, demoHooks+`on quan insert src/core.txt "// qu:gone" "x"`+"\n")
	root := newDemoProject(t, st)
	quanar := filepath.Join(root, "quanar")

	res, err := NewSingle(Single{Kind: quanarch.Quan, Name: "stock", Target: filepath.Join(root, "quans", "stock"),
		Qua: QuaSource{From: LocalQua, Name: "acme-qua", Dir: filepath.Join(root, "packages", "acme-qua")}}, st, false, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Registered) != 0 || len(res.Notes) != 1 || !strings.Contains(res.Notes[0], `has no "// qu:gone" line`) {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, filepath.Join(quanar, "src", "list.txt")); strings.Contains(got, "stock") {
		t.Errorf("a half-changed quanar: %q", got)
	}
}

func TestStackWithoutHooksLeavesANote(t *testing.T) {
	st := hookStack(t, "")
	root := newDemoProject(t, st)

	res, err := NewSingle(Single{Kind: quanarch.Quan, Name: "stock", Target: filepath.Join(root, "quans", "stock"),
		Qua: QuaSource{From: LocalQua, Name: "acme-qua", Dir: filepath.Join(root, "packages", "acme-qua")}}, st, false, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "the stack does not say how; add stock, served on port 5002") {
		t.Errorf("notes = %q", res.Notes)
	}
}

func renderData(name string, port int, qua string) render.Data {
	return render.Data{Name: name, Kind: "quan", Port: port, Quas: []render.Qua{{Name: qua, Version: "1"}}}
}

func TestRenderCommand(t *testing.T) {
	data := render.Data{Name: "ship-qua", Dependency: "file:../packages/ship-qua"}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"npm", "install", "[[", ".Dependency", "]]"}, "npm|install|file:../packages/ship-qua"},
		{[]string{"go", "get", "[[ .Name ]]@latest"}, "go|get|ship-qua@latest"},
		{[]string{"sh", "-c", "echo [[ .Name ]] # done"}, "sh|-c|echo ship-qua # done"},
	} {
		got, err := renderCommand("test", c.args, data)
		if err != nil || strings.Join(got, "|") != c.want {
			t.Errorf("%q = %q, %v; want %q", c.args, got, err, c.want)
		}
	}
}
