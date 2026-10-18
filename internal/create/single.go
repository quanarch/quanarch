package create

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/quanarch/quanarch/internal/conf"
	"github.com/quanarch/quanarch/internal/project"
	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/render"
)

// QuaFrom says where a new quanar, quan or qmock gets its qua.
type QuaFrom int

const (
	// NoQua creates it without a qua; its README says one is needed.
	NoQua QuaFrom = iota
	// LocalQua links a qua that is already on disk.
	LocalQua
	// NewQua creates a qua in the project's packages folder first.
	NewQua
	// PublishedQua depends on a qua from a registry or a URL.
	PublishedQua
)

// QuaSource is the qua a new quanar, quan or qmock is wired to.
type QuaSource struct {
	From QuaFrom
	Name string
	// Dir is the qua's folder, for LocalQua.
	Dir string
	// Spec is a version range or a URL, for PublishedQua.
	Spec string
}

// Single is one quanar, quan, qua or qmock.
type Single struct {
	Kind quanarch.Kind
	Name string
	// Target is the folder to create, absolute.
	Target string
	Qua    QuaSource
}

// Result says what was created beyond the target itself.
type Result struct {
	// Port is the port a quan is served on, or the dev server port of a quanar or qmock.
	Port int
	// Registered lists the files the new quan was added to.
	Registered []string
	// Wired lists the quanars and qmocks the new quan's qua was plugged into.
	Wired []string
	// Notes are things qu could not do itself and the user should.
	Notes []string
}

// Dev server ports of a quanar and a qmock.
const (
	QuanarPort = 5173
	QmockPort  = 5174
)

// NewSingle creates one item. Inside a project it links the project's quas,
// takes the next free port and registers a new quan with the project's quanar.
func NewSingle(s Single, st Stack, run bool, out io.Writer) (Result, error) {
	root, inProject := project.FindRoot(filepath.Dir(s.Target))
	var res Result

	data := render.Data{
		Name:         s.Name,
		Kind:         string(s.Kind),
		Stack:        st.Manifest.Name,
		StackVersion: st.Manifest.Version,
	}
	if inProject {
		data.Project = filepath.Base(root)
	}

	if s.Kind != quanarch.Qua {
		qua, err := resolveQua(s, root, inProject, st, run, out)
		if err != nil {
			return res, err
		}
		if qua != nil {
			data.Quas = []render.Qua{*qua}
		}
	}

	switch s.Kind {
	case quanarch.Quan:
		res.Port = project.FirstQuanPort
		if inProject {
			res.Port = project.NextQuanPort(root)
		}
		data.Port = res.Port
	case quanarch.Quanar, quanarch.Qmock:
		res.Port = QuanarPort
		if s.Kind == quanarch.Qmock {
			res.Port = QmockPort
		}
		data.Port = res.Port
		if inProject {
			for _, q := range project.Quans(root) {
				data.Quans = append(data.Quans, render.Quan{Name: q.Name, Port: q.Port})
			}
		}
	}

	if err := renderPart(s.Kind, s.Target, data, st, run, out); err != nil {
		return res, err
	}

	if s.Kind == quanarch.Quan && inProject {
		for _, name := range []string{"quanar", "qmock"} {
			host := filepath.Join(root, name)
			if _, err := os.Stat(host); err != nil {
				continue
			}
			file := filepath.Join(host, "src", "config", "index.ts")
			ok, err := registerQuan(file, s.Name, res.Port)
			if err != nil {
				return res, err
			}
			if ok {
				res.Registered = append(res.Registered, file)
			}
			// The quan's qua must be plugged into every host that loads it.
			for _, q := range data.Quas {
				dep := q.Dependency
				if q.Path != "" {
					rel, err := filepath.Rel(host, filepath.Join(s.Target, filepath.FromSlash(q.Path)))
					if err != nil {
						return res, err
					}
					dep = "file:" + filepath.ToSlash(rel)
				}
				wired, err := wireQua(host, q.Name, dep)
				if err != nil {
					res.Notes = append(res.Notes, err.Error()+"; plug "+q.Name+" in by hand")
					continue
				}
				if wired {
					res.Wired = append(res.Wired, host)
					if run {
						if err := runSteps([][]string{{"npm", "install"}}, host, out); err != nil {
							return res, err
						}
					}
				}
			}
		}
	}
	return res, nil
}

// resolveQua turns the chosen qua source into template data, creating the qua first if asked.
func resolveQua(s Single, root string, inProject bool, st Stack, run bool, out io.Writer) (*render.Qua, error) {
	switch s.Qua.From {
	case NoQua:
		return nil, nil

	case NewQua:
		if !inProject {
			return nil, fmt.Errorf("a new qua can only be created inside a project")
		}
		dir := filepath.Join(root, "packages", s.Qua.Name)
		data := render.Data{Name: s.Qua.Name, Kind: string(quanarch.Qua), Stack: st.Manifest.Name, StackVersion: st.Manifest.Version, Project: filepath.Base(root)}
		if err := renderPart(quanarch.Qua, dir, data, st, run, out); err != nil {
			return nil, err
		}
		return localQua(s.Target, s.Qua.Name, dir)

	case LocalQua:
		return localQua(s.Target, s.Qua.Name, s.Qua.Dir)

	case PublishedQua:
		version := strings.TrimLeft(s.Qua.Spec, "^~=v")
		if conf.ValidateVersion(version) != nil {
			version = "" // a URL or a range conf.qu cannot express
		}
		return &render.Qua{Name: s.Qua.Name, Version: version, Dependency: s.Qua.Spec}, nil
	}
	return nil, fmt.Errorf("unknown qua source %d", s.Qua.From)
}

func localQua(target, name, dir string) (*render.Qua, error) {
	rel, err := filepath.Rel(target, dir)
	if err != nil {
		return nil, err
	}
	version := "1"
	if p, err := conf.Load(dir); err == nil && p.Version != "" {
		version = p.Version
	}
	rel = filepath.ToSlash(rel)
	return &render.Qua{Name: name, Version: version, Path: rel, Dependency: "file:" + rel}, nil
}

// renderPart writes one template and runs its setup commands.
func renderPart(kind quanarch.Kind, dst string, data render.Data, st Stack, run bool, out io.Writer) error {
	t, err := st.template(kind)
	if err != nil {
		return err
	}
	src := os.DirFS(filepath.Join(st.Dir, filepath.FromSlash(t.Dir)))
	if err := render.Render(src, dst, data); err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	fmt.Fprintf(out, " ✔ %-7s %s\n", kind, dst)
	if !run {
		return nil
	}
	return runSteps(t.Run, dst, out)
}

// registerQuan adds a quan to a host's quan list, just before its closing bracket.
// It reports false when the file does not exist or does not look as generated.
func registerQuan(file, name string, port int) (bool, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return false, nil
	}
	src := string(b)
	if strings.Contains(src, "name: '"+name+"'") {
		return true, nil
	}
	end := strings.LastIndex(src, "\n];")
	if end < 0 {
		return false, nil
	}
	entry := fmt.Sprintf("\n  { name: '%s', url: 'http://localhost:%d/%s.js' },", name, port, name)
	return true, os.WriteFile(file, []byte(src[:end]+entry+src[end:]), 0o644)
}

// NewExample writes a stack's example application to target.
func NewExample(id, name, target string, st Stack, run bool, out io.Writer) error {
	t, ok := st.Manifest.Examples[id]
	if !ok {
		return fmt.Errorf("stack %s has no example %q", st.Manifest.Name, id)
	}
	data := render.Data{Name: name, Kind: "examples", Stack: st.Manifest.Name, StackVersion: st.Manifest.Version, Project: name}
	src := os.DirFS(filepath.Join(st.Dir, filepath.FromSlash(t.Dir)))
	if err := render.Render(src, target, data); err != nil {
		return fmt.Errorf("example %s: %w", id, err)
	}
	fmt.Fprintf(out, " ✔ example %s → %s\n", id, target)
	if !run {
		return nil
	}
	return runSteps(t.Run, target, out)
}
