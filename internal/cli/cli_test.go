package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execute(t *testing.T, name string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(name, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestProgramName(t *testing.T) {
	cases := map[string]string{
		"/usr/local/bin/qu":       "qu",
		"qu.exe":                  "qu",
		"/usr/local/bin/quanarch": "quanarch",
		"quanarch.exe":            "quanarch",
		"/tmp/go-build/cli.test":  "quanarch",
	}
	for arg0, want := range cases {
		if got := programName(arg0); got != want {
			t.Errorf("programName(%q) = %q, want %q", arg0, got, want)
		}
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := execute(t, "qu", "version")
	if code != 0 || !strings.HasPrefix(out, "qu dev ") {
		t.Fatalf("version: code=%d out=%q", code, out)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	src := "quan billing\nqua qua-a@1\nzone admin {\n\tqua qua-b@2\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "conf.qu"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := execute(t, "qu", "check", dir)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	for _, want := range []string{"quan billing", "qua    qua-a@1", "zone   admin", "qua qua-b@2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCheckMissingFile(t *testing.T) {
	code, _, errOut := execute(t, "qu", "check", t.TempDir())
	if code != 1 || !strings.Contains(errOut, "no conf.qu found") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestBareQuShowsHelpWithoutTerminal(t *testing.T) {
	code, out, _ := execute(t, "qu")
	if code != 0 || !strings.Contains(out, "Usage:") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestNewAsksOnlyForWhatIsMissing(t *testing.T) {
	cases := map[string][]string{
		"missing the kind (qu new <kind>), --lang, --dir, the name":                  {"new"},
		"missing --lang, --dir, the name (qu new <kind> <name>), --qua or --new-qua": {"new", "quan"},
		"missing --dir, --qua or --new-qua":                                          {"new", "quan", "billing", "--lang", "react-ts"},
		"missing --quanar, --quan":                                                   {"new", "project", "acme", "--lang", "react-ts", "--dir", ".", "--qua", "acme-qua"},
	}
	for want, args := range cases {
		code, _, errOut := execute(t, "qu", args...)
		if code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("%v: code=%d stderr=%q, want %q", args, code, errOut, want)
		}
	}
}

func TestNewValidatesArguments(t *testing.T) {
	cases := map[string][]string{
		`unknown command "plugin"`:  {"new", "plugin"},
		"must be lowercase":         {"new", "quan", "Billing"},
		"accepts at most 1 arg":     {"new", "quan", "a", "b"},
		`unknown language "cobol"`:  {"new", "quan", "a", "--lang", "cobol"},
		"either --qua or --new-qua": {"new", "quan", "a", "--qua", "x", "--new-qua", "y"},
		"no qua with that name":     {"new", "quan", "a", "--qua", "ghost"},
		"unknown flag: --quanar":    {"new", "quan", "a", "--quanar", "x"},
	}
	for want, args := range cases {
		code, _, errOut := execute(t, "qu", args...)
		if code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("%v: code=%d stderr=%q, want %q", args, code, errOut, want)
		}
	}
}

// writeStack lays out a minimal stack whose templates write a conf.qu.
func writeStack(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"stack.qu":                   "stack react-ts\nversion 1.0.0\ntemplate project t/project\ntemplate qua t/qua\ntemplate quanar t/quanar\ntemplate quan t/quan\n",
		"t/project/conf.qu.qtpl":     "project [[ .Name ]]\n",
		"t/qua/conf.qu.qtpl":         "qua [[ .Name ]]\nversion 1\n",
		"t/quanar/conf.qu.qtpl":      "quanar [[ .Name ]]\n",
		"t/quanar/package.json.qtpl": "{\n  \"dependencies\": {\n  }\n}\n",
		"t/quan/conf.qu.qtpl":        "quan [[ .Name ]][[ range .Quas ]]\nqua [[ .Name ]][[ end ]]\n",
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
	return dir
}

func TestNewWithoutAQuestion(t *testing.T) {
	t.Setenv("QU_STACK_DIR", writeStack(t))
	dir := t.TempDir()

	code, out, errOut := execute(t, "qu", "new", "project", "acme", "--lang", "react-ts", "--dir", dir,
		"--qua", "acme-qua", "--quanar", "acme-quanar", "--quan", "billing")
	if code != 0 {
		t.Fatalf("project: code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("output: %q", out)
	}

	root := filepath.Join(dir, "acme")
	code, _, errOut = execute(t, "qu", "new", "quan", "invoices", "--lang", "react-ts", "--dir", root, "--qua", "acme-qua")
	if code != 0 {
		t.Fatalf("quan: code=%d stderr=%q", code, errOut)
	}
	conf, err := os.ReadFile(filepath.Join(root, "quans", "invoices", "conf.qu"))
	if err != nil || !strings.Contains(string(conf), "qua acme-qua") {
		t.Fatalf("quans/invoices/conf.qu = %q, %v", conf, err)
	}

	code, _, errOut = execute(t, "qu", "new", "quan", "invoices", "--lang", "react-ts", "--dir", root, "--qua", "none")
	if code != 1 || !strings.Contains(errOut, "not empty") {
		t.Errorf("an existing quan was overwritten: code=%d stderr=%q", code, errOut)
	}
}

func TestHelpIsLayered(t *testing.T) {
	_, root, _ := execute(t, "qu", "--help")
	_, newHelp, _ := execute(t, "qu", "new", "--help")
	_, quanHelp, _ := execute(t, "qu", "new", "quan", "--help")
	for name, check := range map[string]struct{ text, want string }{
		"root shows the logo":       {root, "██████╗"},
		"root points at new":        {root, "qu new --help"},
		"new lists the kinds":       {newHelp, "qmock      a stand-in quanar"},
		"new points at kinds":       {newHelp, "qu new <kind> --help"},
		"quan lists its flags":      {quanHelp, "--new-qua"},
		"quan shows a full command": {quanHelp, "qu new quan billing --lang react-ts --dir . --qua acme-qua"},
	} {
		if !strings.Contains(check.text, check.want) {
			t.Errorf("%s: %q not found", name, check.want)
		}
	}
}
