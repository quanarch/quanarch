package conf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/quanarch/quanarch/internal/quanarch"
)

// Project is the meaning of a conf.qu file.
type Project struct {
	Kind    quanarch.Kind
	Name    string
	Version string // optional
	Stack   *Ref   // optional
	// Quas lists the quas a quan uses or a quanar plugs in. Always empty for a qua.
	Quas  []Ref
	Zones []Zone // quans only
}

// Zone is an isolated area inside a quan with its own quas.
type Zone struct {
	Name string
	Quas []Ref
	Line int
}

// Ref names a qua or a stack, optionally pinned to a version: "qua-a@1".
type Ref struct {
	Name    string
	Version string
}

func (r Ref) String() string {
	if r.Version == "" {
		return r.Name
	}
	return r.Name + "@" + r.Version
}

var versionPattern = regexp.MustCompile(`^\d+(\.\d+){0,2}(-[0-9A-Za-z.-]+)?$`)

// ParseRef parses "name" or "name@version".
func ParseRef(s string) (Ref, error) {
	name, version, pinned := strings.Cut(s, "@")
	if err := quanarch.ValidateName(name); err != nil {
		return Ref{}, err
	}
	if pinned {
		if err := ValidateVersion(version); err != nil {
			return Ref{}, err
		}
	}
	return Ref{Name: name, Version: version}, nil
}

// ValidateVersion reports whether v is a version such as 1, 1.2, 1.2.3 or 2.0.0-beta.1.
func ValidateVersion(v string) error {
	if !versionPattern.MatchString(v) {
		return fmt.Errorf("invalid version %q (expected a form like 1, 1.2 or 1.2.3)", v)
	}
	return nil
}

// Load reads and validates the conf.qu file in dir.
func Load(dir string) (*Project, error) {
	path := filepath.Join(dir, FileName)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f, path)
}

// Decode reads and validates a conf.qu file from r. file is used only in error messages.
func Decode(r io.Reader, file string) (*Project, error) {
	directives, err := ParseDirectives(r, file)
	if err != nil {
		return nil, err
	}
	d := decoder{file: file}
	return d.project(directives)
}

type decoder struct {
	file string
}

func (d decoder) errorf(line int, format string, args ...any) error {
	return &Error{File: d.file, Line: line, Msg: fmt.Sprintf(format, args...)}
}

func (d decoder) project(directives []*Directive) (*Project, error) {
	if len(directives) == 0 {
		return nil, &Error{File: d.file, Msg: "file is empty; the first line must declare the project, for example \"quan billing\""}
	}

	header := directives[0]
	kind, err := quanarch.ParseKind(header.Name)
	if err != nil {
		return nil, d.errorf(header.Line, "the first line must declare the project as quanar, quan, qua or qmock, found %q", header.Name)
	}
	if err := d.noBlock(header); err != nil {
		return nil, err
	}
	if len(header.Args) != 1 {
		return nil, d.errorf(header.Line, "%s takes exactly one name", kind)
	}
	if strings.Contains(header.Args[0], "@") {
		return nil, d.errorf(header.Line, "the project name cannot carry a version; use the version directive")
	}
	if err := quanarch.ValidateName(header.Args[0]); err != nil {
		return nil, d.errorf(header.Line, "%v", err)
	}

	p := &Project{Kind: kind, Name: header.Args[0]}
	seen := map[string]int{} // single-use directives -> line
	quas := refSet{}
	zones := map[string]bool{}

	for _, dir := range directives[1:] {
		switch dir.Name {
		case "stack", "version":
			if line, ok := seen[dir.Name]; ok {
				return nil, d.errorf(dir.Line, "%s is already set on line %d", dir.Name, line)
			}
			seen[dir.Name] = dir.Line
			if err := d.oneArg(dir); err != nil {
				return nil, err
			}
			if dir.Name == "version" {
				if err := ValidateVersion(dir.Args[0]); err != nil {
					return nil, d.errorf(dir.Line, "%v", err)
				}
				p.Version = dir.Args[0]
				continue
			}
			ref, err := ParseRef(dir.Args[0])
			if err != nil {
				return nil, d.errorf(dir.Line, "stack: %v", err)
			}
			p.Stack = &ref

		case "qua":
			if kind == quanarch.Qua {
				return nil, d.errorf(dir.Line, "a qua cannot reference other quas")
			}
			ref, err := d.qua(dir, quas)
			if err != nil {
				return nil, err
			}
			p.Quas = append(p.Quas, ref)

		case "zone":
			if kind != quanarch.Quan {
				return nil, d.errorf(dir.Line, "zones are only allowed in a quan")
			}
			zone, err := d.zone(dir)
			if err != nil {
				return nil, err
			}
			if zones[zone.Name] {
				return nil, d.errorf(dir.Line, "zone %q is declared more than once", zone.Name)
			}
			zones[zone.Name] = true
			p.Zones = append(p.Zones, zone)

		case "quan", "quanar", "qmock":
			return nil, d.errorf(dir.Line, "the project is already declared as %s %s on line %d", kind, p.Name, header.Line)

		default:
			return nil, d.errorf(dir.Line, "unknown directive %q", dir.Name)
		}
	}
	return p, nil
}

func (d decoder) zone(dir *Directive) (Zone, error) {
	if len(dir.Args) != 1 {
		return Zone{}, d.errorf(dir.Line, "zone takes exactly one name")
	}
	if err := quanarch.ValidateName(dir.Args[0]); err != nil {
		return Zone{}, d.errorf(dir.Line, "zone: %v", err)
	}
	if !dir.HasBlock {
		return Zone{}, d.errorf(dir.Line, "zone %s needs a block: zone %s { ... }", dir.Args[0], dir.Args[0])
	}

	zone := Zone{Name: dir.Args[0], Line: dir.Line}
	quas := refSet{}
	for _, inner := range dir.Block {
		if inner.Name != "qua" {
			return Zone{}, d.errorf(inner.Line, "only qua is allowed inside a zone, found %q", inner.Name)
		}
		ref, err := d.qua(inner, quas)
		if err != nil {
			return Zone{}, err
		}
		zone.Quas = append(zone.Quas, ref)
	}
	return zone, nil
}

func (d decoder) qua(dir *Directive, seen refSet) (Ref, error) {
	if err := d.oneArg(dir); err != nil {
		return Ref{}, err
	}
	ref, err := ParseRef(dir.Args[0])
	if err != nil {
		return Ref{}, d.errorf(dir.Line, "qua: %v", err)
	}
	if line, ok := seen[ref]; ok {
		return Ref{}, d.errorf(dir.Line, "qua %s is already listed on line %d", ref, line)
	}
	seen[ref] = dir.Line
	return ref, nil
}

func (d decoder) oneArg(dir *Directive) error {
	if err := d.noBlock(dir); err != nil {
		return err
	}
	if len(dir.Args) != 1 {
		return d.errorf(dir.Line, "%s takes exactly one argument", dir.Name)
	}
	return nil
}

func (d decoder) noBlock(dir *Directive) error {
	if dir.HasBlock {
		return d.errorf(dir.Line, "%s does not take a block", dir.Name)
	}
	return nil
}

type refSet map[Ref]int
