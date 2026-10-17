package cli

import (
	"bytes"
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
