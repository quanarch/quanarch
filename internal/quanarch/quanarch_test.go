package quanarch

import (
	"strings"
	"testing"
)

func TestParseKind(t *testing.T) {
	for _, k := range Kinds {
		got, err := ParseKind(string(k))
		if err != nil || got != k {
			t.Errorf("ParseKind(%q) = %q, %v", k, got, err)
		}
	}
	if _, err := ParseKind("plugin"); err == nil {
		t.Error("ParseKind(\"plugin\") succeeded, want error")
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"billing", "billing-report", "qua-a", "v2", "a1-b2-c3"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"", "Billing", "billing_report", "-billing", "billing-", "billing--report", "2fa", "bil ling", strings.Repeat("a", maxNameLength+1)}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}
}
