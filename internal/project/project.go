// Package project reads an existing Quanarch project: its root, its quas and its quans.
package project

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/quanarch/quanarch/internal/conf"
	"github.com/quanarch/quanarch/internal/quanarch"
)

// FirstQuanPort is the port the first quan of a project is served on when its stack does not say.
const FirstQuanPort = 4001

// FindRoot returns the root of the project dir belongs to: the nearest folder,
// dir itself or above, whose conf.qu declares a project. ok is false outside a project.
func FindRoot(dir string) (root string, ok bool) {
	dir = filepath.Clean(dir)
	for {
		if p, err := conf.Load(dir); err == nil && p.Kind == quanarch.Project {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Qua is a qua inside a project's packages folder.
type Qua struct {
	Name    string
	Version string
	Dir     string
}

// Quas lists the quas in root/packages, by name.
func Quas(root string) []Qua {
	var quas []Qua
	for _, dir := range subdirs(filepath.Join(root, "packages")) {
		if p, err := conf.Load(dir); err == nil && p.Kind == quanarch.Qua {
			quas = append(quas, Qua{Name: p.Name, Version: p.Version, Dir: dir})
		}
	}
	return quas
}

// Quan is a quan inside a project's quans folder.
type Quan struct {
	Name string
	Port int
	Dir  string
	// Quas are the quas the quan uses, as its conf.qu lists them.
	Quas []conf.Ref
}

var portFlag = regexp.MustCompile(`--port\s+(\d+)`)

// Quans lists the quans in root/quans with the port each one is served on, by name.
func Quans(root string) []Quan {
	var quans []Quan
	for _, dir := range subdirs(filepath.Join(root, "quans")) {
		p, err := conf.Load(dir)
		if err != nil || p.Kind != quanarch.Quan {
			continue
		}
		port := p.Port
		if port == 0 {
			port = legacyWatchPort(dir)
		}
		quans = append(quans, Quan{Name: p.Name, Port: port, Dir: dir, Quas: p.Quas})
	}
	return quans
}

// NextQuanPort returns a port no quan of the project uses yet, starting at first.
func NextQuanPort(root string, first int) int {
	port := first
	if port == 0 {
		port = FirstQuanPort
	}
	for _, q := range Quans(root) {
		if q.Port >= port {
			port = q.Port + 1
		}
	}
	return port
}

// Place returns where a new item named name goes when dir is chosen as its location.
// Inside a project root, each kind has its own place; anywhere else it goes in dir.
func Place(kind quanarch.Kind, dir, name string) string {
	root, ok := FindRoot(dir)
	if !ok || filepath.Clean(dir) != root {
		return filepath.Join(dir, name)
	}
	switch kind {
	case quanarch.Quan:
		return filepath.Join(root, "quans", name)
	case quanarch.Qua:
		return filepath.Join(root, "packages", name)
	case quanarch.Quanar:
		if !exists(filepath.Join(root, "quanar")) {
			return filepath.Join(root, "quanar")
		}
	case quanarch.Qmock:
		if !exists(filepath.Join(root, "qmock")) {
			return filepath.Join(root, "qmock")
		}
	}
	return filepath.Join(root, name)
}

// legacyWatchPort reads the port of a quan's watch script; 0 if there is none.
// Quans write their port in conf.qu; this only finds the port of a quan created
// by an older qu, from the npm script those stacks used.
func legacyWatchPort(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return 0
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(b, &pkg) != nil {
		return 0
	}
	m := portFlag.FindStringSubmatch(pkg.Scripts["watch"])
	if m == nil {
		return 0
	}
	port := 0
	for _, c := range m[1] {
		port = port*10 + int(c-'0')
	}
	return port
}

func subdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
