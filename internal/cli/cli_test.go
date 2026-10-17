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
