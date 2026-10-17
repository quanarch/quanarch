package render

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

var data = Data{Name: "billing-report", Kind: "quan", Stack: "react-ts", StackVersion: "1.0.0"}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRender(t *testing.T) {
	src := fstest.MapFS{
		"conf.qu.qtpl":                {Data: []byte("[[ .Kind ]] [[ .Name ]]\nstack [[ .Stack ]]@[[ .StackVersion ]]\n")},
		"src/[[ pascal .Name ]].tsx":  {Data: []byte("export const x = {{ raw }};\n")},
		"src/App.vue":                 {Data: []byte("<p>{{ message }}</p>\n")},
		"README.md.qtpl":              {Data: []byte("# [[ title .Name ]]\n[[ camel .Name ]] [[ snake .Name ]] [[ upper .Kind ]]\n")},
		"scripts/dev.sh":              {Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		".git/config":                 {Data: []byte("ignored")},
		"assets/[[ .Name ]]/logo.png": {Data: []byte{0x89, 'P', 'N', 'G'}},
	}
	dst := filepath.Join(t.TempDir(), "out")
	if err := Render(src, dst, data); err != nil {
		t.Fatal(err)
	}

	checks := map[string]string{
		"conf.qu":                        "quan billing-report\nstack react-ts@1.0.0\n",
		"src/BillingReport.tsx":          "export const x = {{ raw }};\n",
		"src/App.vue":                    "<p>{{ message }}</p>\n",
		"README.md":                      "# Billing Report\nbillingReport billing_report QUAN\n",
		"assets/billing-report/logo.png": "\x89PNG",
	}
	for rel, want := range checks {
		if got := readFile(t, filepath.Join(dst, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".git was copied: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dst, "scripts", "dev.sh"))
		if err != nil || info.Mode().Perm()&0o100 == 0 {
			t.Errorf("dev.sh lost its executable bit: %v %v", info, err)
		}
	}
}

func TestRenderIntoEmptyDirectory(t *testing.T) {
	dst := t.TempDir()
	if err := Render(fstest.MapFS{"a": {Data: []byte("a")}}, dst, data); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "a")); got != "a" {
		t.Errorf("a = %q", got)
	}
}

func TestRenderRefusesNonEmptyDestination(t *testing.T) {
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(dst, "keep"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Render(fstest.MapFS{"a": {Data: []byte("a")}}, dst, data)
	if !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if got := readFile(t, filepath.Join(dst, "keep")); got != "mine" {
		t.Errorf("existing file changed: %q", got)
	}
}

func TestRenderLeavesNothingOnError(t *testing.T) {
	parent := t.TempDir()
	dst := filepath.Join(parent, "out")
	src := fstest.MapFS{
		"ok.txt":       {Data: []byte("fine")},
		"bad.txt.qtpl": {Data: []byte("[[ .Missing ]]")},
	}
	err := Render(src, dst, data)
	if err == nil {
		t.Fatal("Render succeeded, want error")
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Errorf("left behind %d entries in %s", len(entries), parent)
	}
}

func TestRenderPathErrors(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"escapes":   {"[[ \"..\" ]]/x": {Data: []byte("x")}},
		"separator": {"[[ \"a/b\" ]]": {Data: []byte("x")}},
		"collision": {"a.qtpl": {Data: []byte("x")}, "a": {Data: []byte("y")}},
		"empty":     {"[[ \"\" ]]": {Data: []byte("x")}},
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			err := Render(src, filepath.Join(t.TempDir(), "out"), data)
			if err == nil {
				t.Fatal("Render succeeded, want error")
			}
			if name == "collision" && !strings.Contains(err.Error(), "both render to") {
				t.Errorf("err = %v", err)
			}
		})
	}
}
