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
	// Port is the port a quan is served on, or the dev server port of a quanar or qmock; 0 if the stack does not say.
	Port int
	// Registered lists the quanars and qmocks the new quan was added to.
	Registered []string
	// Wired lists the quanars and qmocks the new quan's qua was plugged into.
	Wired []string
	// Notes are things qu could not do itself and the user should.
	Notes []string
}

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
		res.Port = st.Port(quanarch.Quan)
		if res.Port == 0 {
			res.Port = project.FirstQuanPort
		}
		if inProject {
			res.Port = project.NextQuanPort(root, res.Port)
		}
		data.Port = res.Port
	case quanarch.Quanar, quanarch.Qmock:
		res.Port = st.Port(s.Kind)
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
			if err := takeIn(st, host, s, data, run, out, &res); err != nil {
				return res, err
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

// takeIn adds a new quan to a quanar or qmock the way the stack declares: first its
// quas the host does not have yet, then the quan itself. What cannot be done automatically
// becomes a note.
func takeIn(st Stack, host string, s Single, data render.Data, run bool, out io.Writer, res *Result) error {
	var quas []render.Qua
	for _, q := range data.Quas {
		dep := q.Dependency
		if q.Path != "" {
			// From here on the qua is seen from the host, not from the new quan.
			rel, err := filepath.Rel(host, filepath.Join(s.Target, filepath.FromSlash(q.Path)))
			if err != nil {
				return err
			}
			q.Path = filepath.ToSlash(rel)
			dep = "file:" + q.Path
		}
		q.Dependency = dep
		quas = append(quas, q)

		if hasQua(host, q.Name) {
			continue
		}
		quaData := render.Data{Name: q.Name, Kind: string(quanarch.Qua), Stack: data.Stack, StackVersion: data.StackVersion,
			Project: data.Project, Version: q.Version, Dependency: dep, Quas: []render.Qua{q}}
		if _, err := applyHooks(st, quanarch.Qua, host, quaData, run, out); err != nil {
			res.Notes = append(res.Notes, hookNote(err, "plug "+q.Name+" into "+host))
			continue
		}
		if err := addQuaToConf(host, conf.Ref{Name: q.Name, Version: q.Version}); err != nil {
			return err
		}
		res.Wired = append(res.Wired, host)
	}

	quanData := data
	quanData.Quas = quas
	changed, err := applyHooks(st, quanarch.Quan, host, quanData, run, out)
	if err != nil {
		res.Notes = append(res.Notes, hookNote(err, fmt.Sprintf("add %s, served on port %d, to %s", s.Name, data.Port, host)))
		return nil
	}
	if changed {
		res.Registered = append(res.Registered, host)
	}
	return nil
}

func hookNote(err error, todo string) string {
	return err.Error() + "; " + todo + " by hand"
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
