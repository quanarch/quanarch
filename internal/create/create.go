// Package create writes new projects from a stack's templates.
package create

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/quanarch/quanarch/internal/project"
	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/render"
	"github.com/quanarch/quanarch/internal/stack"
)

// Stack is a stack available on disk.
type Stack struct {
	Dir      string
	Manifest *stack.Manifest
}

func (s Stack) template(kind quanarch.Kind) (stack.Template, error) {
	t, ok := s.Manifest.Templates[kind]
	if !ok {
		return stack.Template{}, fmt.Errorf("stack %s has no template for %s", s.Manifest.Name, kind)
	}
	return t, nil
}

// Project is a quanar, a qua and a quan created together.
type Project struct {
	// Dir is the folder the project folder is created in.
	Dir    string
	Name   string
	Qua    string
	Quanar string
	Quan   string
}

// part is one piece of a project: what to render, where, and with which data.
type part struct {
	kind quanarch.Kind
	dir  string // relative to the project root
	data render.Data
}

// NewProject writes the project and installs its dependencies. Setup commands
// run only when run is true.
func NewProject(p Project, st Stack, run bool, out io.Writer) error {
	root := filepath.Join(p.Dir, p.Name)
	quas := func(path string) []render.Qua {
		return []render.Qua{{Name: p.Qua, Version: "1", Path: path, Dependency: "file:" + path}}
	}
	common := render.Data{Stack: st.Manifest.Name, StackVersion: st.Manifest.Version, Project: p.Name}
	quanPort := st.Port(quanarch.Quan)
	if quanPort == 0 {
		quanPort = project.FirstQuanPort
	}

	parts := []part{
		{kind: quanarch.Project, dir: ".", data: with(common, func(d *render.Data) {
			d.Name, d.Kind = p.Name, string(quanarch.Project)
			d.Quas = quas("packages/" + p.Qua)
			d.Quans = []render.Quan{{Name: p.Quan, Port: quanPort, Quas: d.Quas}}
		})},
		{kind: quanarch.Qua, dir: "packages/" + p.Qua, data: with(common, func(d *render.Data) {
			d.Name, d.Kind = p.Qua, string(quanarch.Qua)
		})},
		{kind: quanarch.Quanar, dir: "quanar", data: with(common, func(d *render.Data) {
			d.Name, d.Kind = p.Quanar, string(quanarch.Quanar)
			d.Quas = quas("../packages/" + p.Qua)
			d.Quans = []render.Quan{{Name: p.Quan, Port: quanPort, Quas: d.Quas}}
			d.Port = st.Port(quanarch.Quanar)
		})},
		{kind: quanarch.Quan, dir: "quans/" + p.Quan, data: with(common, func(d *render.Data) {
			d.Name, d.Kind = p.Quan, string(quanarch.Quan)
			d.Quas = quas("../../packages/" + p.Qua)
			d.Port = quanPort
		})},
	}

	for _, pt := range parts {
		t, err := st.template(pt.kind)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, filepath.FromSlash(pt.dir))
		if pt.dir == "." {
			dst = root
		}
		src := os.DirFS(filepath.Join(st.Dir, filepath.FromSlash(t.Dir)))
		if err := render.Render(src, dst, pt.data); err != nil {
			return fmt.Errorf("%s: %w", pt.kind, err)
		}
		fmt.Fprintf(out, " ✔ %-7s %s\n", pt.kind, displayPath(root, dst))
	}

	if !run {
		return nil
	}
	// The qua first: the quanar and the quan depend on it.
	for _, pt := range parts {
		t, _ := st.template(pt.kind)
		if err := runSteps(t.Run, filepath.Join(root, filepath.FromSlash(pt.dir)), out); err != nil {
			return err
		}
	}
	return nil
}

// runSteps runs a template's setup commands in dir.
func runSteps(steps [][]string, dir string, out io.Writer) error {
	for _, args := range steps {
		fmt.Fprintf(out, "\n → %s  (%s)\n", strings.Join(args, " "), dir)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%q in %s failed: %w", strings.Join(args, " "), dir, err)
		}
	}
	return nil
}

func with(d render.Data, f func(*render.Data)) render.Data {
	f(&d)
	return d
}

func displayPath(root, p string) string {
	rel, err := filepath.Rel(filepath.Dir(root), p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}
