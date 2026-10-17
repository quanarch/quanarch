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

// writeStack creates a local stack with a quan template and returns its path.
func writeStack(t *testing.T, run string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"stack.qu":                    "stack demo\nversion 1.2.0\ntemplate quan templates/quan" + run + "\n",
		"templates/quan/conf.qu.qtpl": "[[ .Kind ]] [[ .Name ]]\nstack [[ .Stack ]]@[[ .StackVersion ]]\n",
		"templates/quan/src/[[ pascal .Name ]].ts": "export {};\n",
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

func TestNewFromLocalStack(t *testing.T) {
	stackDir := writeStack(t, " {\n\trun go version\n}")
	dst := filepath.Join(t.TempDir(), "billing")

	code, out, errOut := execute(t, "qu", "new", "quan", "billing-report", "--stack", stackDir, "--dir", dst)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "→ go version") || !strings.Contains(out, "Created quan billing-report in "+dst+" from demo 1.2.0") {
		t.Errorf("output:\n%s", out)
	}
	conf, err := os.ReadFile(filepath.Join(dst, "conf.qu"))
	if err != nil || string(conf) != "quan billing-report\nstack demo@1.2.0\n" {
		t.Errorf("conf.qu = %q, %v", conf, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "src", "BillingReport.ts")); err != nil {
		t.Error(err)
	}
}

func TestNewNoRun(t *testing.T) {
	stackDir := writeStack(t, " {\n\trun does-not-exist\n}")
	dst := filepath.Join(t.TempDir(), "x")
	code, out, errOut := execute(t, "qu", "new", "quan", "x", "--stack", stackDir, "--dir", dst, "--no-run")
	if code != 0 || strings.Contains(out, "→") {
		t.Fatalf("code=%d out=%q stderr=%q", code, out, errOut)
	}

	code, _, errOut = execute(t, "qu", "new", "quan", "y", "--stack", stackDir, "--dir", filepath.Join(t.TempDir(), "y"))
	if code != 1 || !strings.Contains(errOut, `but "does-not-exist" failed`) {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestNewErrors(t *testing.T) {
	stackDir := writeStack(t, "")
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"new", "quan", "x"}, "--stack is required (official stacks: react-ts)"},
		{[]string{"new", "quanar", "x", "--stack", stackDir}, "stack demo has no template for quanar"},
		{[]string{"new", "quan", "x", "--stack", t.TempDir()}, "is not a stack: no stack.qu found"},
		{[]string{"new", "quan", "x", "--stack", "unknown-stack"}, `unknown stack "unknown-stack"`},
	}
	for _, c := range cases {
		code, _, errOut := execute(t, "qu", c.args...)
		if code != 1 || !strings.Contains(errOut, c.want) {
			t.Errorf("%v: code=%d stderr=%q, want %q", c.args, code, errOut, c.want)
		}
	}
}

func TestNewRejectsInvalidName(t *testing.T) {
	code, _, errOut := execute(t, "qu", "new", "quan", "Billing")
	if code != 1 || !strings.Contains(errOut, "kebab-case") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestNewRequiresName(t *testing.T) {
	code, _, errOut := execute(t, "qu", "new", "qua")
	if code != 1 || !strings.Contains(errOut, "accepts 1 arg") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestNewListsKinds(t *testing.T) {
	code, out, _ := execute(t, "qu", "new")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	for _, kind := range []string{"quan", "quanar", "qua"} {
		if !strings.Contains(out, "  "+kind+" ") {
			t.Errorf("help does not list %q:\n%s", kind, out)
		}
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
