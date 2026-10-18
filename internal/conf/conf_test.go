package conf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/quanarch/quanarch/internal/quanarch"
)

func decode(t *testing.T, src string) (*Project, error) {
	t.Helper()
	return Decode(strings.NewReader(src), FileName)
}

func TestDecodeQuan(t *testing.T) {
	src := `# Billing quan
quan billing
stack react-ts@1
version 0.3.0

qua qua-a@1   # every user
qua qua-b

zone admin {
	qua qua-b@2
}
`
	got, err := decode(t, src)
	if err != nil {
		t.Fatal(err)
	}
	want := &Project{
		Kind:    quanarch.Quan,
		Name:    "billing",
		Version: "0.3.0",
		Stack:   &Ref{Name: "react-ts", Version: "1"},
		Quas:    []Ref{{"qua-a", "1"}, {"qua-b", ""}},
		Zones:   []Zone{{Name: "admin", Quas: []Ref{{"qua-b", "2"}}, Line: 9}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestDecodeQuanarAndQua(t *testing.T) {
	quanar, err := decode(t, "quanar main\nqua qua-a@1\nqua qua-a@2\n")
	if err != nil {
		t.Fatal(err)
	}
	if quanar.Kind != quanarch.Quanar || len(quanar.Quas) != 2 {
		t.Errorf("quanar: %+v", quanar)
	}

	qua, err := decode(t, "qua qua-a\nversion 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if qua.Kind != quanarch.Qua || qua.Name != "qua-a" || qua.Version != "2" {
		t.Errorf("qua: %+v", qua)
	}
}

func TestDecodeErrors(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"empty", "# nothing\n", "file is empty"},
		{"no header", "stack react-ts\n", `conf.qu:1: the first line must declare the project`},
		{"header with version", "quan billing@1\n", "conf.qu:1: the project name cannot carry a version"},
		{"bad name", "quan Billing\n", "conf.qu:1: name \"Billing\" must be lowercase kebab-case"},
		{"second header", "quan a\nquanar b\n", "conf.qu:2: the project is already declared as quan a on line 1"},
		{"unknown", "quan a\nplugin b\n", `conf.qu:2: unknown directive "plugin"`},
		{"duplicate stack", "quan a\nstack x\nstack y\n", "conf.qu:3: stack is already set on line 2"},
		{"bad version", "quan a\nversion one\n", `conf.qu:2: invalid version "one"`},
		{"duplicate qua", "quan a\nqua q@1\nqua q@1\n", "conf.qu:3: qua q@1 is already listed on line 2"},
		{"qua in qua", "qua a\nqua b\n", "conf.qu:2: a qua cannot reference other quas"},
		{"zone in quanar", "quanar a\nzone z {\n}\n", "conf.qu:2: zones are only allowed in a quan"},
		{"zone without block", "quan a\nzone z\n", "conf.qu:2: zone z needs a block"},
		{"stack in zone", "quan a\nzone z {\n\tstack x\n}\n", `conf.qu:3: only qua is allowed inside a zone, found "stack"`},
		{"duplicate zone", "quan a\nzone z {\n}\nzone z {\n}\n", `conf.qu:4: zone "z" is declared more than once`},
		{"block on qua", "quan a\nqua q {\n}\n", "conf.qu:2: qua does not take a block"},
		{"extra args", "quan a\nqua q r\n", "conf.qu:2: qua takes exactly one argument"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := decode(t, c.src)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestParseDirectives(t *testing.T) {
	src := "a \"x y\" z # comment\nb {\n\tc \"say \\\"hi\\\"\"\n}\n"
	got, err := ParseDirectives(strings.NewReader(src), FileName)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d directives", len(got))
	}
	if got[0].Name != "a" || !reflect.DeepEqual(got[0].Args, []string{"x y", "z"}) {
		t.Errorf("first: %+v", got[0])
	}
	if !got[1].HasBlock || len(got[1].Block) != 1 || got[1].Block[0].Args[0] != `say "hi"` || got[1].Block[0].Line != 3 {
		t.Errorf("block: %+v", got[1])
	}
}

func TestParseDirectivesErrors(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"unclosed block", "a {\nb\n", `conf.qu:1: block "a" is never closed`},
		{"stray close", "}\n", `conf.qu:1: unexpected "}"`},
		{"close with args", "a {\n} x\n", `conf.qu:2: "}" must be on a line of its own`},
		{"brace in middle", "a { b\n", `conf.qu:1: unexpected "{"`},
		{"lonely open", "{\n", `conf.qu:1: "{" must follow a directive`},
		{"unterminated quote", "a \"b\n", "conf.qu:1: unterminated quoted string"},
		{"quote in word", "a b\"c\"\n", "conf.qu:1: unexpected quote"},
		{"text after quote", "a \"b\"c\n", "conf.qu:1: closing quote must be followed by whitespace"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDirectives(strings.NewReader(c.src), FileName)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	for _, s := range []string{"qua-a", "qua-a@1", "qua-a@1.2", "qua-a@1.2.3", "qua-a@2.0.0-beta.1"} {
		ref, err := ParseRef(s)
		if err != nil || ref.String() != s {
			t.Errorf("ParseRef(%q) = %v, %v", s, ref, err)
		}
	}
	for _, s := range []string{"", "@1", "qua-a@", "qua-a@v1", "qua-a@1.2.3.4", "Qua@1"} {
		if _, err := ParseRef(s); err == nil {
			t.Errorf("ParseRef(%q) succeeded, want error", s)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("quanar main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir)
	if err != nil || p.Name != "main" {
		t.Fatalf("Load = %+v, %v", p, err)
	}

	_, err = Load(t.TempDir())
	if !os.IsNotExist(err) {
		t.Errorf("Load on empty dir: %v, want not-exist error", err)
	}
}

func TestDecodePort(t *testing.T) {
	p, err := Decode(strings.NewReader("quan billing\nport 4002\n"), FileName)
	if err != nil || p.Port != 4002 {
		t.Fatalf("Decode = %+v, %v", p, err)
	}
	for src, want := range map[string]string{
		"qua q\nport 1\n":            "a qua has no port",
		"project p\nport 1\n":        "a project has no port",
		"quan b\nport x\n":           `invalid port "x"`,
		"quan b\nport 0\n":           `invalid port "0"`,
		"quanar r\nport 1\nport 2\n": "port is already set on line 2",
	} {
		if _, err := Decode(strings.NewReader(src), FileName); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", src, err, want)
		}
	}
}
