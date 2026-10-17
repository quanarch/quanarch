// Package quanarch defines the core concepts of the Quanarch architecture.
// It knows nothing about languages or frameworks.
package quanarch

import "fmt"

// Kind is something qu can create: one of the three building blocks of a
// Quanarch system, or a qmock to develop quans against.
type Kind string

const (
	// Project is a quanar, a qua and a quan created together as one project.
	Project Kind = "project"
	// Quan is a unit of business logic. It cannot run on its own and lives inside a quanar.
	Quan Kind = "quan"
	// Quanar hosts quans and plugs in the quas they are allowed to use. It holds no business logic.
	Quanar Kind = "quanar"
	// Qua is a contract: the only window a quan has into the quanar.
	Qua Kind = "qua"
	// Qmock is a stand-in quanar for developing and trying quans.
	Qmock Kind = "qmock"
)

// Kinds lists every kind in a stable order.
var Kinds = []Kind{Project, Quanar, Quan, Qua, Qmock}

// ParseKind returns the kind named s.
func ParseKind(s string) (Kind, error) {
	for _, k := range Kinds {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown kind %q (expected project, quanar, quan, qua or qmock)", s)
}

// Summary returns a one-line description of the kind.
func (k Kind) Summary() string {
	switch k {
	case Project:
		return "a quanar, a qua and a quan together, ready to run"
	case Quan:
		return "a unit of business logic that lives inside a quanar"
	case Quanar:
		return "the host that runs quans and plugs in their quas"
	case Qua:
		return "a contract between quans and a quanar"
	case Qmock:
		return "a stand-in quanar for developing quans"
	}
	return ""
}
