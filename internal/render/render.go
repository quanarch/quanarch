// Package render turns a template directory into a new project directory.
//
// Files ending in ".qtpl" are executed as Go text/templates with "[[" and "]]"
// as delimiters, and the suffix is dropped. The delimiters differ from the usual
// "{{ }}" so templates for Vue, Angular, Handlebars or Go itself need no escaping.
// Every other file is copied byte for byte. Path segments may contain template
// actions too, for example "src/[[ pascal .Name ]].tsx".
package render

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
)

// TemplateSuffix marks files whose contents are executed as templates.
const TemplateSuffix = ".qtpl"

// Data is available to templates as the dot value.
type Data struct {
	Name         string // kebab-case project name, for example "billing-report"
	Kind         string // project, quanar, quan, qua or qmock
	Stack        string
	StackVersion string
	Quas         []Qua // quas a quan uses or a quanar plugs in
	// Project is the name of the project the new code belongs to, if any.
	Project string
	// Quans lists the quans a quanar loads.
	Quans []Quan
	// Port is the port a quan is served on while it is developed.
	Port int
	// Version and Dependency describe a qua being plugged into a quanar or qmock.
	// Dependency is what that host's package manager needs: "file:../packages/qua-a", "^1.0.0" or a URL.
	Version    string
	Dependency string
}

// Quan is a quan a quanar loads.
type Quan struct {
	Name string
	// Port is the port the quan is served on while it is developed.
	Port int
}

// Qua is a qua referenced by the new project.
type Qua struct {
	Name    string // for example "qua-a"
	Version string // may be empty
	// Path is the slash-separated path from the new project to a qua on disk,
	// for example "../qua-a". Empty when the qua comes from a registry.
	Path string
	// Dependency is what a package manager needs to fetch the qua: "file:../qua-a", "^1.0.0" or a URL.
	Dependency string
}

// Funcs are the helper functions available to templates.
var Funcs = template.FuncMap{
	"pascal": func(s string) string { return joinWords(s, "", capitalize) },
	"camel": func(s string) string {
		words := splitWords(s)
		for i := 1; i < len(words); i++ {
			words[i] = capitalize(words[i])
		}
		return strings.Join(words, "")
	},
	"snake": func(s string) string { return joinWords(s, "_", nil) },
	"kebab": func(s string) string { return joinWords(s, "-", nil) },
	"title": func(s string) string { return joinWords(s, " ", capitalize) },
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
}

// splitWords splits s on "-", "_" and spaces and lowercases each word.
func splitWords(s string) []string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return words
}

func joinWords(s, sep string, f func(string) string) string {
	words := splitWords(s)
	if f != nil {
		for i, w := range words {
			words[i] = f(w)
		}
	}
	return strings.Join(words, sep)
}

func capitalize(w string) string {
	r, size := utf8.DecodeRuneInString(w)
	if r == utf8.RuneError {
		return w
	}
	return string(unicode.ToUpper(r)) + w[size:]
}

// ErrExists is returned when the destination already exists and is not an empty directory.
var ErrExists = errors.New("destination already exists and is not empty")

// Render writes the template in src to the directory dst. dst must not exist or
// must be an empty directory. Nothing is left behind if rendering fails.
func Render(src fs.FS, dst string, data Data) error {
	if err := checkDestination(dst); err != nil {
		return err
	}

	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".qu-render-*")
	if err != nil {
		return err
	}
	if err := renderInto(src, tmp, data); err != nil {
		os.RemoveAll(tmp)
		return err
	}

	// An empty destination directory is replaced; checkDestination rejected anything else.
	if err := os.Remove(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

func checkDestination(dst string) error {
	entries, err := os.ReadDir(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return err
	case len(entries) > 0:
		return fmt.Errorf("%s: %w", dst, ErrExists)
	}
	return nil
}

func renderInto(src fs.FS, root string, data Data) error {
	written := map[string]string{} // rendered path -> source path

	return fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symbolic links are not supported in templates", p)
		}

		rel, err := renderPath(p, data)
		if err != nil {
			return err
		}
		if prev, ok := written[rel]; ok {
			return fmt.Errorf("%s and %s both render to %s", prev, p, rel)
		}
		written[rel] = p
		target := filepath.Join(root, filepath.FromSlash(rel))

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: only regular files are supported in templates", p)
		}

		content, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, TemplateSuffix) {
			if content, err = execute(p, string(content), data); err != nil {
				return err
			}
		}

		mode := fs.FileMode(0o644)
		if info, err := d.Info(); err == nil && info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		return os.WriteFile(target, content, mode)
	})
}

// renderPath renders each segment of a slash-separated template path.
func renderPath(p string, data Data) (string, error) {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if strings.Contains(seg, "[[") {
			out, err := execute(p, seg, data)
			if err != nil {
				return "", err
			}
			seg = string(out)
		}
		if i == len(segments)-1 {
			seg = strings.TrimSuffix(seg, TemplateSuffix)
		}
		if seg == "" || seg == "." || seg == ".." || strings.ContainsAny(seg, `/\`) {
			return "", fmt.Errorf("%s: renders to an invalid path segment %q", p, seg)
		}
		segments[i] = seg
	}
	return path.Join(segments...), nil
}

// Text renders one template string. name is used only in error messages.
func Text(name, text string, data Data) (string, error) {
	out, err := execute(name, text, data)
	return string(out), err
}

func execute(name, text string, data Data) ([]byte, error) {
	t, err := template.New(name).Delims("[[", "]]").Funcs(Funcs).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
