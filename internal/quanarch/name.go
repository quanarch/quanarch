package quanarch

import (
	"fmt"
	"regexp"
)

const maxNameLength = 64

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// ValidateName reports whether name can be used for a quan, quanar or qua.
// Names are lowercase kebab-case so they map cleanly onto package names,
// directories and URLs in every stack.
func ValidateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("name is required")
	case len(name) > maxNameLength:
		return fmt.Errorf("name %q is longer than %d characters", name, maxNameLength)
	case !namePattern.MatchString(name):
		return fmt.Errorf("name %q must be lowercase kebab-case, for example %q", name, "billing-report")
	}
	return nil
}
