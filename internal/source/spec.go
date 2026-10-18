// Package source locates stacks and fetches them into a local cache.
package source

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/quanarch/quanarch/internal/conf"
)

// Registry maps official stack names to their repositories.
var Registry = map[string]string{
	"react-ts": "https://github.com/quanarch/quanarch-react",
	"node-ts":  "https://github.com/quanarch/quanarch-node",
}

// Spec says where a stack comes from, as given to --stack.
type Spec struct {
	// Local is set for a stack on disk; Location is then an absolute path.
	Local bool
	// Location is a directory for a local stack, otherwise a git URL.
	Location string
	// Version selects a release by its tag. Empty means the latest release.
	Version string
}

func (s Spec) String() string {
	if s.Version == "" {
		return s.Location
	}
	return s.Location + "@" + s.Version
}

// ParseSpec understands three forms:
//
//	react-ts, react-ts@1            an official stack
//	./my-stack, /abs/path           a stack on disk
//	github.com/org/repo@1.2         a stack in a git repository
//	https://host/repo.git           (also git@host:org/repo)
func ParseSpec(s string) (Spec, error) {
	switch {
	case s == "":
		return Spec{}, fmt.Errorf("stack is empty")

	case IsLocalPath(s):
		abs, err := filepath.Abs(s)
		if err != nil {
			return Spec{}, err
		}
		return Spec{Local: true, Location: abs}, nil

	case strings.Contains(s, "://") || strings.HasPrefix(s, "git@") || strings.Contains(firstSegment(s), "."):
		location, version := splitVersion(s)
		if version != "" {
			if err := conf.ValidateVersion(version); err != nil {
				return Spec{}, err
			}
		}
		if !strings.Contains(location, "://") && !strings.HasPrefix(location, "git@") {
			location = "https://" + location
		}
		return Spec{Location: location, Version: version}, nil
	}

	ref, err := conf.ParseRef(s)
	if err != nil {
		return Spec{}, fmt.Errorf("stack %q: %w", s, err)
	}
	url, ok := Registry[ref.Name]
	if !ok {
		return Spec{}, fmt.Errorf("unknown stack %q (official stacks: %s)", ref.Name, strings.Join(Official(), ", "))
	}
	return Spec{Location: url, Version: ref.Version}, nil
}

// Official returns the names of the official stacks in order.
func Official() []string {
	names := make([]string, 0, len(Registry))
	for name := range Registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsLocalPath reports whether s names something on disk rather than a registry name or URL.
func IsLocalPath(s string) bool {
	return s == "." || s == ".." ||
		strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, `.\`) || strings.HasPrefix(s, `..\`) ||
		filepath.IsAbs(s) || strings.HasPrefix(s, "/")
}

func firstSegment(s string) string {
	seg, _, _ := strings.Cut(s, "/")
	return seg
}

// splitVersion splits "location@version". An "@" before the last "/" or ":"
// belongs to the location, as in git@github.com:org/repo.
func splitVersion(s string) (location, version string) {
	at := strings.LastIndex(s, "@")
	if at < 0 || at < strings.LastIndexAny(s, "/:") {
		return s, ""
	}
	return s[:at], s[at+1:]
}
