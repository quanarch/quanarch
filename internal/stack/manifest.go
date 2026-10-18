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
//		port 4001
//		start npm run watch
//	}
//	template quanar templates/quanar
//	template qua templates/qua
//
//	on quan insert src/config/index.ts "// qu:quans" "{ name: '[[ .Name ]]' },"
//	on qua run npm install [[ .Dependency ]]
//
// The engine knows no language. Everything that depends on one, the ports, the
// commands that start a part, and how a new quan or qua is added to a quanar,
// is declared here by the stack.
package stack

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
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
	// Examples are complete applications, by ID.
	Examples map[string]Template
	// Hooks say how a quanar or qmock takes in a quan or a qua added to the project later.
	Hooks []Hook
}

// Hook is one "on" directive: what to do in every quanar and qmock of a project
// when a quan is added to it, or when a quan brings a qua they do not have yet.
type Hook struct {
	// Event is quan or qua.
	Event quanarch.Kind
	// File and Marker say where to insert: the lines go just above the line that holds
	// exactly Marker. Both are empty for a hook that runs a command.
	File   string
	Marker string
	// Lines are inserted, each one a template.
	Lines []string
	// Run is a command to run in the quanar or qmock, each argument a template.
	Run  []string
	Line int
}

// Template is the template a stack provides for one kind.
type Template struct {
	Kind quanarch.Kind
	// Dir is the template directory, relative to the stack root, using forward slashes.
	Dir string
	// Run lists commands to run in the new project after it is created.
	Run [][]string
	// Port is the port the part is served on while it is developed: a quanar's or
	// qmock's server, a quan's watch. For a quan it is the first port of a project. 0 if not given.
	Port int
	// Start is the command that runs the part while it is developed. Empty if not given.
	Start []string
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

	m := &Manifest{Name: header.Args[0], Templates: map[quanarch.Kind]Template{}, Examples: map[string]Template{}}
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

		case "on":
			h, err := decodeHook(d, errorf)
			if err != nil {
				return nil, err
			}
			m.Hooks = append(m.Hooks, h)

		case "example":
			id, t, err := decodeExample(d, errorf)
			if err != nil {
				return nil, err
			}
			if _, ok := m.Examples[id]; ok {
				return nil, errorf(d.Line, "example %s is already declared", id)
			}
			m.Examples[id] = t

		default:
			return nil, errorf(d.Line, "unknown directive %q", d.Name)
		}
	}
	if len(m.Templates) == 0 && len(m.Examples) == 0 {
		return nil, errorf(0, "stack %s declares no templates", m.Name)
	}
	return m, nil
}

// decodeExample reads "example <id> <dir> [{ run ... }]".
func decodeExample(d *conf.Directive, errorf func(int, string, ...any) error) (string, Template, error) {
	if len(d.Args) != 2 {
		return "", Template{}, errorf(d.Line, "example takes an ID and a directory, for example \"example advanced examples/advanced\"")
	}
	if err := quanarch.ValidateName(d.Args[0]); err != nil {
		return "", Template{}, errorf(d.Line, "example: %v", err)
	}
	dir, err := cleanRelPath(d.Args[1])
	if err != nil {
		return "", Template{}, errorf(d.Line, "example directory: %v", err)
	}
	t := Template{Dir: dir}
	for _, inner := range d.Block {
		switch {
		case inner.Name == "run" && !inner.HasBlock && len(inner.Args) > 0:
			t.Run = append(t.Run, inner.Args)
		case inner.Name == "start" && !inner.HasBlock && len(inner.Args) > 0 && t.Start == nil:
			t.Start = inner.Args
		default:
			return "", Template{}, errorf(inner.Line, "only run and one start are allowed inside an example")
		}
	}
	return d.Args[0], t, nil
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
		switch inner.Name {
		case "run":
			if inner.HasBlock || len(inner.Args) == 0 {
				return Template{}, errorf(inner.Line, "run takes a command, for example \"run npm install\"")
			}
			t.Run = append(t.Run, inner.Args)
		case "port":
			if t.Port != 0 {
				return Template{}, errorf(inner.Line, "port is already set")
			}
			port, err := ParsePort(inner.Args)
			if inner.HasBlock || err != nil {
				return Template{}, errorf(inner.Line, "port takes one number between 1 and 65535, for example \"port 4001\"")
			}
			t.Port = port
		case "start":
			if t.Start != nil {
				return Template{}, errorf(inner.Line, "start is already set")
			}
			if inner.HasBlock || len(inner.Args) == 0 {
				return Template{}, errorf(inner.Line, "start takes a command, for example \"start npm run dev\"")
			}
			t.Start = inner.Args
		default:
			return Template{}, errorf(inner.Line, "only run, port and start are allowed inside a template, found %q", inner.Name)
		}
	}
	return t, nil
}

// ParsePort reads a single port number.
func ParsePort(args []string) (int, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("one port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", args[0])
	}
	return port, nil
}

// decodeHook reads one of
//
//	on quan|qua insert <file> <marker> <line>
//	on quan|qua insert <file> <marker> {
//		line <line>
//		...
//	}
//	on quan|qua run <command...>
func decodeHook(d *conf.Directive, errorf func(int, string, ...any) error) (Hook, error) {
	usage := `on takes quan or qua, then "insert <file> <marker> <line>" or "run <command>"`
	if len(d.Args) < 2 {
		return Hook{}, errorf(d.Line, "%s", usage)
	}
	h := Hook{Line: d.Line}
	switch d.Args[0] {
	case string(quanarch.Quan), string(quanarch.Qua):
		h.Event = quanarch.Kind(d.Args[0])
	default:
		return Hook{}, errorf(d.Line, "on: %q is not an event; use quan or qua", d.Args[0])
	}

	switch d.Args[1] {
	case "run":
		if d.HasBlock || len(d.Args) < 3 {
			return Hook{}, errorf(d.Line, "on %s run takes a command", h.Event)
		}
		h.Run = d.Args[2:]

	case "insert":
		args := d.Args[2:]
		if len(args) < 2 || len(args) > 3 || (len(args) == 3) == d.HasBlock {
			return Hook{}, errorf(d.Line, "on %s insert takes a file, a marker and either one line or a block of line directives", h.Event)
		}
		file, err := cleanRelPath(args[0])
		if err != nil {
			return Hook{}, errorf(d.Line, "on %s insert: %v", h.Event, err)
		}
		if strings.TrimSpace(args[1]) == "" {
			return Hook{}, errorf(d.Line, "on %s insert: the marker cannot be empty", h.Event)
		}
		h.File, h.Marker = file, strings.TrimSpace(args[1])
		if len(args) == 3 {
			h.Lines = []string{args[2]}
		}
		for _, inner := range d.Block {
			if inner.Name != "line" || inner.HasBlock || len(inner.Args) > 1 {
				return Hook{}, errorf(inner.Line, `only "line <text>" is allowed inside on %s insert`, h.Event)
			}
			text := ""
			if len(inner.Args) == 1 {
				text = inner.Args[0]
			}
			h.Lines = append(h.Lines, text)
		}
		if len(h.Lines) == 0 {
			return Hook{}, errorf(d.Line, "on %s insert has nothing to insert", h.Event)
		}

	default:
		return Hook{}, errorf(d.Line, "%s", usage)
	}
	return h, nil
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
