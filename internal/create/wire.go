package create

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quanarch/quanarch/internal/render"
)

var (
	pascal = render.Funcs["pascal"].(func(string) string)
	camel  = render.Funcs["camel"].(func(string) string)
)

// wireQua plugs a qua into a quanar or qmock so the quans that use it can reach it:
// a package.json dependency, the module shared on window, and a core passed to init().
// It edits the host at the "qu:" markers its template leaves. wired is false when
// the host already has the qua.
func wireQua(host, name, dependency string) (wired bool, err error) {
	pkg := filepath.Join(host, "package.json")
	src, err := os.ReadFile(pkg)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(src), fmt.Sprintf("%q:", name)) {
		return false, nil
	}

	global, core := pascal(name), camel(name)+"Core"
	edits := []struct {
		file, marker, text string
	}{
		{"src/main.tsx", "// qu:qua-imports", fmt.Sprintf("import * as %s from '%s';", global, name)},
		{"src/main.tsx", "// qu:qua-window-types", fmt.Sprintf("%s: typeof %s;", global, global)},
		{"src/main.tsx", "// qu:qua-window", fmt.Sprintf("window.%s = %s;", global, global)},
		{"src/core/qua-core/index.ts", "// qu:qua-imports", fmt.Sprintf("import * as %s from '%s';", global, name)},
		{"src/core/qua-core/index.ts", "// qu:qua-cores", fmt.Sprintf("const %s: %s.QuanarCore = {\n  increment: (value) => value + 1,\n};\n", core, global)},
		{"src/core/qua-core/index.ts", "// qu:qua-init", fmt.Sprintf("if (%s.isInitialized()) %s.reset();\n%s.init(%s);", global, global, global, core)},
	}

	// Check every marker before touching anything, so a host is never half-wired.
	files := map[string]string{}
	for _, e := range edits {
		if _, ok := files[e.file]; !ok {
			b, err := os.ReadFile(filepath.Join(host, filepath.FromSlash(e.file)))
			if err != nil {
				return false, fmt.Errorf("%s cannot be wired automatically: %w", host, err)
			}
			files[e.file] = string(b)
		}
		if markerAt(files[e.file], e.marker) < 0 {
			return false, fmt.Errorf("%s cannot be wired automatically: %s has no %q line", host, e.file, e.marker)
		}
	}
	const depsKey = "\"dependencies\": {"
	at := strings.Index(string(src), depsKey)
	if at < 0 {
		return false, fmt.Errorf("%s cannot be wired automatically: package.json has no dependencies", host)
	}

	for _, e := range edits {
		files[e.file] = insertBefore(files[e.file], e.marker, e.text)
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(host, filepath.FromSlash(file)), []byte(content), 0o644); err != nil {
			return false, err
		}
	}
	end := at + len(depsKey)
	entry := fmt.Sprintf("\n    %q: %q,", name, dependency)
	if err := os.WriteFile(pkg, []byte(string(src[:end])+entry+string(src[end:])), 0o644); err != nil {
		return false, err
	}
	return true, nil
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
