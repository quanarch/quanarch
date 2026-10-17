// Package quanarch defines the core concepts of the Quanarch architecture.
// It knows nothing about languages or frameworks.
package quanarch

import "fmt"

// Kind is one of the three building blocks of a Quanarch system.
type Kind string

const (
	// Quan is a unit of business logic. It cannot run on its own and lives inside a quanar.
	Quan Kind = "quan"
	// Quanar hosts quans and plugs in the quas they are allowed to use. It holds no business logic.
	Quanar Kind = "quanar"
	// Qua is a contract: the only window a quan has into the quanar.
	Qua Kind = "qua"
)

// Kinds lists every kind in a stable order.
var Kinds = []Kind{Quan, Quanar, Qua}

// ParseKind returns the kind named s.
func ParseKind(s string) (Kind, error) {
	for _, k := range Kinds {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown kind %q (expected quan, quanar or qua)", s)
}

// Summary returns a one-line description of the kind.
func (k Kind) Summary() string {
	switch k {
	case Quan:
		return "a unit of business logic that lives inside a quanar"
	case Quanar:
		return "the host that runs quans and plugs in their quas"
	case Qua:
		return "a contract between quans and a quanar"
	}
	return ""
}
