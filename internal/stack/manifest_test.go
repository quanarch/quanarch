package stack

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/quanarch/quanarch/internal/quanarch"
)

func TestDecodeManifest(t *testing.T) {
	src := `stack react-ts
version 1.0.0
title "React + TypeScript"

template quan templates/quan {
	run npm install
	run git init
}
template quanar ./templates/quanar/
`
	got, err := DecodeManifest(strings.NewReader(src), ManifestFile)
	if err != nil {
		t.Fatal(err)
	}
	want := &Manifest{
		Name:    "react-ts",
		Version: "1.0.0",
		Title:   "React + TypeScript",
		Templates: map[quanarch.Kind]Template{
			quanarch.Quan:   {Kind: quanarch.Quan, Dir: "templates/quan", Run: [][]string{{"npm", "install"}, {"git", "init"}}},
			quanarch.Quanar: {Kind: quanarch.Quanar, Dir: "templates/quanar"},
		},
		Examples: map[string]Template{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestDecodeManifestErrors(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"no header", "template quan t\n", "stack.qu:1: the first line must declare the stack"},
		{"empty", "", "the first line must declare the stack"},
		{"no templates", "stack s\n", "stack s declares no templates"},
		{"bad kind", "stack s\ntemplate plugin t\n", `stack.qu:2: unknown kind "plugin"`},
		{"duplicate kind", "stack s\ntemplate quan a\ntemplate quan b\n", "stack.qu:3: a template for quan is already declared"},
		{"missing dir", "stack s\ntemplate quan\n", "stack.qu:2: template takes a kind and a directory"},
		{"escaping dir", "stack s\ntemplate quan ../x\n", "must stay inside the stack"},
		{"absolute dir", "stack s\ntemplate quan /x\n", "must be a relative path"},
		{"backslash dir", "stack s\ntemplate quan a\\b\n", "must be a relative path"},
		{"bad version", "stack s\nversion x\ntemplate quan t\n", `stack.qu:2: invalid version "x"`},
		{"empty run", "stack s\ntemplate quan t {\n\trun\n}\n", "stack.qu:3: run takes a command"},
		{"other in template", "stack s\ntemplate quan t {\n\tqua x\n}\n", `only run, port and start are allowed inside a template`},
		{"bad port", "stack s\ntemplate quan t {\n\tport 70000\n}\n", "stack.qu:3: port takes one number"},
		{"two ports", "stack s\ntemplate quan t {\n\tport 1\n\tport 2\n}\n", "stack.qu:4: port is already set"},
		{"empty start", "stack s\ntemplate quan t {\n\tstart\n}\n", "stack.qu:3: start takes a command"},
		{"bad event", "stack s\ntemplate quan t\non quanar run x\n", `stack.qu:3: on: "quanar" is not an event`},
		{"bad action", "stack s\ntemplate quan t\non quan copy a b\n", "stack.qu:3: on takes quan or qua"},
		{"insert without text", "stack s\ntemplate quan t\non quan insert a.ts \"// qu:x\"\n", "stack.qu:3: on quan insert takes a file, a marker and either one line"},
		{"insert with both", "stack s\ntemplate quan t\non quan insert a.ts m x {\n\tline y\n}\n", "stack.qu:3: on quan insert takes"},
		{"insert escaping", "stack s\ntemplate quan t\non qua insert ../a m x\n", "must stay inside the stack"},
		{"other in insert", "stack s\ntemplate quan t\non qua insert a m {\n\trun x\n}\n", `stack.qu:4: only "line <text>" is allowed`},
		{"run without command", "stack s\ntemplate quan t\non qua run\n", "stack.qu:3: on qua run takes a command"},
		{"unknown", "stack s\nfoo bar\n", `stack.qu:2: unknown directive "foo"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DecodeManifest(strings.NewReader(c.src), ManifestFile)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte("stack s\ntemplate qua t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil || m.Templates[quanarch.Qua].Dir != "t" {
		t.Fatalf("LoadManifest = %+v, %v", m, err)
	}
}

func TestDecodeExamples(t *testing.T) {
	src := "stack s\ntemplate quan t\nexample advanced examples/advanced {\n\trun npm install\n}\n"
	m, err := DecodeManifest(strings.NewReader(src), ManifestFile)
	if err != nil {
		t.Fatal(err)
	}
	ex, ok := m.Examples["advanced"]
	if !ok || ex.Dir != "examples/advanced" || len(ex.Run) != 1 {
		t.Fatalf("examples = %+v", m.Examples)
	}
	for _, bad := range []string{"example x\n", "example Bad dir\n", "example a ../x\n", "example a d\nexample a e\n"} {
		if _, err := DecodeManifest(strings.NewReader("stack s\n"+bad), ManifestFile); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestDecodePortsStartsAndHooks(t *testing.T) {
	src := `stack s
template quan t/quan {
	run npm install
	port 4001
	start npm run watch
}
example advanced e {
	start make run
}

on quan insert src/config/index.ts "// qu:quans" "{ name: '[[ .Name ]]' },"
on qua insert src/core.ts "// qu:cores" {
	line "const [[ camel .Name ]] = {"
	line
	line "};"
}
on qua run npm install [[ .Dependency ]]
`
	m, err := DecodeManifest(strings.NewReader(src), ManifestFile)
	if err != nil {
		t.Fatal(err)
	}
	quan := m.Templates[quanarch.Quan]
	if quan.Port != 4001 || !reflect.DeepEqual(quan.Start, []string{"npm", "run", "watch"}) {
		t.Errorf("quan template = %+v", quan)
	}
	if got := m.Examples["advanced"].Start; !reflect.DeepEqual(got, []string{"make", "run"}) {
		t.Errorf("example start = %v", got)
	}
	want := []Hook{
		{Event: quanarch.Quan, File: "src/config/index.ts", Marker: "// qu:quans", Lines: []string{"{ name: '[[ .Name ]]' },"}, Line: 11},
		{Event: quanarch.Qua, File: "src/core.ts", Marker: "// qu:cores", Lines: []string{"const [[ camel .Name ]] = {", "", "};"}, Line: 12},
		// Words are kept as written; a template action is rendered over the whole command later.
		{Event: quanarch.Qua, Run: []string{"npm", "install", "[[", ".Dependency", "]]"}, Line: 17},
	}
	if !reflect.DeepEqual(m.Hooks, want) {
		t.Errorf("hooks\ngot  %#v\nwant %#v", m.Hooks, want)
	}
}
