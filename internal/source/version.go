package source

import (
	"fmt"
	"strconv"
	"strings"
)

// semver is a parsed release tag such as v1.2.3 or 2.0.0-beta.1.
type semver struct {
	tag                 string
	major, minor, patch int
	pre                 string
}

func parseTag(tag string) (semver, bool) {
	s := strings.TrimPrefix(tag, "v")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return semver{}, false
		}
		nums[i] = n
	}
	return semver{tag: tag, major: nums[0], minor: nums[1], patch: nums[2], pre: pre}, true
}

func (v semver) less(w semver) bool {
	if v.major != w.major {
		return v.major < w.major
	}
	if v.minor != w.minor {
		return v.minor < w.minor
	}
	if v.patch != w.patch {
		return v.patch < w.patch
	}
	// A release ranks above its pre-releases.
	if (v.pre == "") != (w.pre == "") {
		return v.pre != ""
	}
	return v.pre < w.pre
}

// matches reports whether v satisfies a constraint such as "", "1", "1.2" or "1.2.3-beta.1".
// Partial constraints never select pre-releases; an exact constraint selects exactly that tag.
func (v semver) matches(constraint string) bool {
	if constraint == "" {
		return v.pre == ""
	}
	core, pre, hasPre := strings.Cut(constraint, "-")
	parts := strings.Split(core, ".")
	have := []int{v.major, v.minor, v.patch}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n != have[i] {
			return false
		}
	}
	if len(parts) == 3 {
		return hasPre == (v.pre != "") && pre == v.pre
	}
	return v.pre == ""
}

// selectTag returns the highest tag that satisfies the constraint.
func selectTag(tags []string, constraint string) (string, error) {
	var best *semver
	for _, t := range tags {
		v, ok := parseTag(t)
		if !ok || !v.matches(constraint) {
			continue
		}
		if best == nil || best.less(v) {
			best = &v
		}
	}
	if best == nil {
		if constraint == "" {
			return "", errNoReleases
		}
		return "", fmt.Errorf("no release matches version %s", constraint)
	}
	return best.tag, nil
}
