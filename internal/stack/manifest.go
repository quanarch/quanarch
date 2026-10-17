// Package stack reads stacks: collections of templates for one language or framework.
//
// A stack is a directory with a stack.qu manifest at its root:
//
//	stack react-ts
//	version 1.0.0
//	title "React + TypeScript"
//
//	template quan templates/quan {
//		run npm install
//	}
//	template quanar templates/quanar
//	template qua templates/qua
package stack

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/quanarch/quanarch/internal/conf"
	"github.com/quanarch/quanarch/internal/quanarch"
)

// ManifestFile is the name of the file at the root of every stack.
const ManifestFile = "stack.qu"

// Manifest describes a stack.
type Manifest struct {
	Name      string
	Version   string // optional
	Title     string // optional, human-readable
	Templates map[quanarch.Kind]Template
}

// Template is the template a stack provides for one kind.
type Template struct {
	Kind quanarch.Kind
	// Dir is the template directory, relative to the stack root, using forward slashes.
	Dir string
	// Run lists commands to run in the new project after it is created.
	Run [][]string
}

// LoadManifest reads the stack.qu file in dir.
func LoadManifest(dir string) (*Manifest, error) {
	file := filepath.Join(dir, ManifestFile)
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodeManifest(f, file)
}

// DecodeManifest reads a stack.qu file from r. file is used only in error messages.
func DecodeManifest(r io.Reader, file string) (*Manifest, error) {
	directives, err := conf.ParseDirectives(r, file)
	if err != nil {
		return nil, err
	}
	errorf := func(line int, format string, args ...any) error {
		return &conf.Error{File: file, Line: line, Msg: fmt.Sprintf(format, args...)}
	}

	if len(directives) == 0 || directives[0].Name != "stack" {
		line := 0
		if len(directives) > 0 {
			line = directives[0].Line
		}
		return nil, errorf(line, `the first line must declare the stack, for example "stack react-ts"`)
	}
	header := directives[0]
	if header.HasBlock || len(header.Args) != 1 {
		return nil, errorf(header.Line, "stack takes exactly one name")
	}
	if err := quanarch.ValidateName(header.Args[0]); err != nil {
		return nil, errorf(header.Line, "%v", err)
	}

	m := &Manifest{Name: header.Args[0], Templates: map[quanarch.Kind]Template{}}
	seen := map[string]int{}
	for _, d := range directives[1:] {
		switch d.Name {
		case "version", "title":
			if line, ok := seen[d.Name]; ok {
				return nil, errorf(d.Line, "%s is already set on line %d", d.Name, line)
			}
			seen[d.Name] = d.Line
			if d.HasBlock || len(d.Args) != 1 {
				return nil, errorf(d.Line, "%s takes exactly one argument", d.Name)
			}
			if d.Name == "title" {
				m.Title = d.Args[0]
				continue
			}
			if err := conf.ValidateVersion(d.Args[0]); err != nil {
				return nil, errorf(d.Line, "%v", err)
			}
			m.Version = d.Args[0]

		case "template":
			t, err := decodeTemplate(d, errorf)
			if err != nil {
				return nil, err
			}
			if _, ok := m.Templates[t.Kind]; ok {
				return nil, errorf(d.Line, "a template for %s is already declared", t.Kind)
			}
			m.Templates[t.Kind] = t

		default:
			return nil, errorf(d.Line, "unknown directive %q", d.Name)
		}
	}
	if len(m.Templates) == 0 {
		return nil, errorf(0, "stack %s declares no templates", m.Name)
	}
	return m, nil
}

func decodeTemplate(d *conf.Directive, errorf func(int, string, ...any) error) (Template, error) {
	if len(d.Args) != 2 {
		return Template{}, errorf(d.Line, "template takes a kind and a directory, for example \"template quan templates/quan\"")
	}
	kind, err := quanarch.ParseKind(d.Args[0])
	if err != nil {
		return Template{}, errorf(d.Line, "%v", err)
	}
	dir, err := cleanRelPath(d.Args[1])
	if err != nil {
		return Template{}, errorf(d.Line, "template directory: %v", err)
	}

	t := Template{Kind: kind, Dir: dir}
	for _, inner := range d.Block {
		if inner.Name != "run" {
			return Template{}, errorf(inner.Line, "only run is allowed inside a template, found %q", inner.Name)
		}
		if inner.HasBlock || len(inner.Args) == 0 {
			return Template{}, errorf(inner.Line, "run takes a command, for example \"run npm install\"")
		}
		t.Run = append(t.Run, inner.Args)
	}
	return t, nil
}

// cleanRelPath accepts only relative, forward-slash paths that stay inside the stack.
func cleanRelPath(p string) (string, error) {
	if p == "" || strings.Contains(p, `\`) || path.IsAbs(p) || filepath.VolumeName(p) != "" {
		return "", fmt.Errorf("%q must be a relative path using forward slashes", p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%q must stay inside the stack", p)
	}
	return clean, nil
}
