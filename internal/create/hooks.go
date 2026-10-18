package create

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/quanarch/quanarch/internal/conf"
	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/render"
	"github.com/quanarch/quanarch/internal/stack"
)

// errNoHooks means the stack does not say how a quanar takes in a quan or a qua.
var errNoHooks = errors.New("the stack does not say how")

// Port is the port a stack gives a part while it is developed; 0 if it does not say.
func (s Stack) Port(kind quanarch.Kind) int {
	return s.Manifest.Templates[kind].Port
}

// Start is the command a stack gives to run a part while it is developed; nil if it does not say.
func (s Stack) Start(kind quanarch.Kind) []string {
	return s.Manifest.Templates[kind].Start
}

func (s Stack) hooks(event quanarch.Kind) []stack.Hook {
	var hooks []stack.Hook
	for _, h := range s.Manifest.Hooks {
		if h.Event == event {
			hooks = append(hooks, h)
		}
	}
	return hooks
}

// applyHooks does what the stack declares for event in host, a quanar or qmock folder.
// Every file and marker is checked before anything is written, so a host is never left
// half-changed. Lines already in place are not inserted again. changed is false when
// there was nothing to do.
func applyHooks(st Stack, event quanarch.Kind, host string, data render.Data, run bool, out io.Writer) (changed bool, err error) {
	hooks := st.hooks(event)
	if len(hooks) == 0 {
		return false, errNoHooks
	}

	type insert struct {
		file, marker string
		lines        []string
	}
	var (
		inserts []insert
		runs    [][]string
		files   = map[string]string{}
	)
	for _, h := range hooks {
		name := fmt.Sprintf("%s:%d", stack.ManifestFile, h.Line)
		if h.Run != nil {
			cmd, err := renderCommand(name, h.Run, data)
			if err != nil {
				return false, err
			}
			runs = append(runs, cmd)
			continue
		}
		lines, err := renderAll(name, h.Lines, data)
		if err != nil {
			return false, err
		}
		if _, ok := files[h.File]; !ok {
			b, err := os.ReadFile(filepath.Join(host, filepath.FromSlash(h.File)))
			if err != nil {
				return false, fmt.Errorf("%s cannot be changed automatically: %w", host, err)
			}
			files[h.File] = string(b)
		}
		if markerAt(files[h.File], h.Marker) < 0 {
			return false, fmt.Errorf("%s cannot be changed automatically: %s has no %q line", host, h.File, h.Marker)
		}
		inserts = append(inserts, insert{h.File, h.Marker, lines})
	}

	written := map[string]bool{}
	for _, in := range inserts {
		if contains(files[in.file], in.lines) {
			continue
		}
		files[in.file] = insertBefore(files[in.file], in.marker, strings.Join(in.lines, "\n"))
		written[in.file] = true
	}
	for file := range written {
		if err := os.WriteFile(filepath.Join(host, filepath.FromSlash(file)), []byte(files[file]), 0o644); err != nil {
			return false, err
		}
	}
	changed = len(written) > 0
	if run && len(runs) > 0 {
		if err := runSteps(runs, host, out); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

func renderAll(name string, texts []string, data render.Data) ([]string, error) {
	out := make([]string, len(texts))
	for i, text := range texts {
		s, err := render.Text(name, text, data)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// renderCommand renders a command as one line, so a template action may span several
// words ("[[ .Dependency ]]" needs no quotes), then splits it the way stack.qu splits words.
func renderCommand(name string, args []string, data render.Data) ([]string, error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t#\"") {
			a = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(a) + `"`
		}
		quoted[i] = a
	}
	line, err := render.Text(name, strings.Join(quoted, " "), data)
	if err != nil {
		return nil, err
	}
	directives, err := conf.ParseDirectives(strings.NewReader("run "+line), name)
	if err != nil || len(directives) != 1 || len(directives[0].Args) == 0 {
		return nil, fmt.Errorf("%s: the command renders to %q", name, line)
	}
	return directives[0].Args, nil
}

// hasQua reports whether host's conf.qu already lists qua.
func hasQua(host, qua string) bool {
	p, err := conf.Load(host)
	if err != nil {
		return false
	}
	for _, ref := range p.Quas {
		if ref.Name == qua {
			return true
		}
	}
	return false
}

// addQuaToConf records in host's conf.qu that qua is plugged in.
func addQuaToConf(host string, ref conf.Ref) error {
	file := filepath.Join(host, conf.FileName)
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	src := strings.TrimRight(string(b), "\n") + "\n"
	return os.WriteFile(file, []byte(src+"qua "+ref.String()+"\n"), 0o644)
}

// contains reports whether lines already appear, one after another, in src.
// Indentation is ignored, since insertBefore indents lines like their marker.
func contains(src string, lines []string) bool {
	want := make([]string, 0, len(lines))
	for _, l := range lines {
		want = append(want, strings.TrimSpace(l))
	}
	have := strings.Split(src, "\n")
	for i := 0; i+len(want) <= len(have); i++ {
		match := true
		for j, w := range want {
			if strings.TrimSpace(have[i+j]) != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// markerAt returns where the line holding exactly marker (after indentation)
// starts, or -1. A marker that only starts a longer line does not count.
func markerAt(src, marker string) int {
	start := 0
	for _, line := range strings.SplitAfter(src, "\n") {
		if strings.TrimSpace(line) == marker {
			return start
		}
		start += len(line)
	}
	return -1
}

// insertBefore puts text, indented like the marker line, on the lines just above the marker.
func insertBefore(src, marker, text string) string {
	lineStart := markerAt(src, marker)
	line := src[lineStart:]
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	var b strings.Builder
	for _, l := range strings.Split(text, "\n") {
		if l != "" {
			b.WriteString(indent + l)
		}
		b.WriteString("\n")
	}
	return src[:lineStart] + b.String() + src[lineStart:]
}
