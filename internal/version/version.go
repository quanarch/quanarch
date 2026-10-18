// Package version holds build information injected at link time.
package version

import (
	"fmt"
	"runtime"
	"strings"
)

// Set with -ldflags "-X github.com/quanarch/quanarch/internal/version.Version=..." at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a single-line description of the build.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s %s/%s)",
		Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Label is the version as shown to people: "v0.1.0", or "development build".
func Label() string {
	if Version == "dev" {
		return "development build"
	}
	return "v" + strings.TrimPrefix(Version, "v")
}
