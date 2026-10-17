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
		{"other in template", "stack s\ntemplate quan t {\n\tqua x\n}\n", `only run is allowed inside a template`},
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
