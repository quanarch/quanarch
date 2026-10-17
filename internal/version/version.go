// Package version holds build information injected at link time.
package version

import (
	"fmt"
	"runtime"
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
