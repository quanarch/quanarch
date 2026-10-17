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
		"kind, language, location, name must be chosen": {"new"},
		"language, location, name must be chosen":       {"new", "quan"},
		"language, location must be chosen":             {"new", "qmock", "dev-quanar"},
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
		`unknown kind "plugin"`: {"new", "plugin"},
		"must be lowercase":     {"new", "quan", "Billing"},
		"accepts at most 2 arg": {"new", "quan", "a", "b"},
	}
	for want, args := range cases {
		code, _, errOut := execute(t, "qu", args...)
		if code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("%v: code=%d stderr=%q, want %q", args, code, errOut, want)
		}
	}
}
