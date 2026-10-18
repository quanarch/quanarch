package source

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSpec(t *testing.T) {
	cases := []struct {
		in   string
		want Spec
	}{
		{"react-ts", Spec{Location: Registry["react-ts"]}},
		{"react-ts@1", Spec{Location: Registry["react-ts"], Version: "1"}},
		{"node-ts", Spec{Location: Registry["node-ts"]}},
		{"python", Spec{Location: Registry["python"]}},
		{"github.com/acme/stack", Spec{Location: "https://github.com/acme/stack"}},
		{"github.com/acme/stack@1.2", Spec{Location: "https://github.com/acme/stack", Version: "1.2"}},
		{"https://git.acme.dev/stack.git@2.0.0", Spec{Location: "https://git.acme.dev/stack.git", Version: "2.0.0"}},
		{"git@github.com:acme/stack", Spec{Location: "git@github.com:acme/stack"}},
		{"git@github.com:acme/stack@3", Spec{Location: "git@github.com:acme/stack", Version: "3"}},
		{"file:///srv/stack@1", Spec{Location: "file:///srv/stack", Version: "1"}},
	}
	for _, c := range cases {
		got, err := ParseSpec(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseSpec(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}

	local, err := ParseSpec("./my-stack")
	if err != nil || !local.Local || !filepath.IsAbs(local.Location) || filepath.Base(local.Location) != "my-stack" {
		t.Errorf("ParseSpec(./my-stack) = %+v, %v", local, err)
	}

	for _, bad := range []string{"", "nope", "react-ts@x", "github.com/acme/stack@v1", "Bad"} {
		if _, err := ParseSpec(bad); err == nil {
			t.Errorf("ParseSpec(%q) succeeded, want error", bad)
		}
	}
}

func TestSelectTag(t *testing.T) {
	tags := []string{"v1.0.0", "v1.2.0", "v1.10.1", "v2.0.0-beta.1", "v2.0.0-beta.2", "notes", "v3", "v01.0.0"}
	cases := map[string]string{
		"":             "v1.10.1",
		"1":            "v1.10.1",
		"1.2":          "v1.2.0",
		"1.0.0":        "v1.0.0",
		"2.0.0-beta.1": "v2.0.0-beta.1",
	}
	for constraint, want := range cases {
		got, err := selectTag(tags, constraint)
		if err != nil || got != want {
			t.Errorf("selectTag(%q) = %q, %v; want %q", constraint, got, err, want)
		}
	}
	for _, constraint := range []string{"2", "1.3", "3"} {
		if got, err := selectTag(tags, constraint); err == nil {
			t.Errorf("selectTag(%q) = %q, want error", constraint, got)
		}
	}
	if _, err := selectTag([]string{"latest"}, ""); err != errNoReleases {
		t.Errorf("selectTag without releases: %v", err)
	}
}

func TestCacheName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/quanarch/quanarch-react": "github.com_quanarch_quanarch-react",
		"git@github.com:acme/stack.git":              "github.com_acme_stack",
		"file:///tmp/x y/stack":                      "tmp_x_y_stack",
	}
	for in, want := range cases {
		if got := cacheName(in); got != want {
			t.Errorf("cacheName(%q) = %q, want %q", in, got, want)
		}
	}
}

// newRepo creates a git repository with a stack.qu that records its version,
// tagging each version given.
func newRepo(t *testing.T, versions ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--quiet", "--initial-branch=main")
	commit := func(content string) {
		if err := os.WriteFile(filepath.Join(dir, "stack.qu"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "stack.qu")
		run("commit", "--quiet", "-m", content)
	}
	if len(versions) == 0 {
		commit("head")
	}
	for _, v := range versions {
		commit(v)
		run("tag", v)
	}
	path := filepath.ToSlash(dir)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // C:/... on Windows
	}
	return "file://" + path
}

func fetch(t *testing.T, f *Fetcher, location, version string) (Fetched, string) {
	t.Helper()
	got, err := f.Fetch(context.Background(), Spec{Location: location, Version: version})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(got.Dir, "stack.qu"))
	if err != nil {
		t.Fatal(err)
	}
	return got, string(b)
}

func TestFetchGit(t *testing.T) {
	url := newRepo(t, "v1.0.0", "v1.1.0", "v2.0.0")
	f := &Fetcher{CacheDir: t.TempDir()}

	got, content := fetch(t, f, url, "")
	if got.Ref != "v2.0.0" || content != "v2.0.0" {
		t.Errorf("latest: %+v %q", got, content)
	}
	if _, err := os.Stat(filepath.Join(got.Dir, ".git")); !os.IsNotExist(err) {
		t.Errorf("cache keeps .git: %v", err)
	}

	got, content = fetch(t, f, url, "1")
	if got.Ref != "v1.1.0" || content != "v1.1.0" {
		t.Errorf("@1: %+v %q", got, content)
	}

	// A cached release is served without touching the repository's files again.
	if err := os.WriteFile(filepath.Join(got.Dir, "marker"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	again, _ := fetch(t, f, url, "1")
	if _, err := os.Stat(filepath.Join(again.Dir, "marker")); err != nil {
		t.Errorf("release was fetched twice: %v", err)
	}

	if _, err := f.Fetch(context.Background(), Spec{Location: url, Version: "3"}); err == nil || !strings.Contains(err.Error(), "no release matches version 3") {
		t.Errorf("@3: %v", err)
	}
}

func TestFetchGitWithoutReleases(t *testing.T) {
	url := newRepo(t)
	f := &Fetcher{CacheDir: t.TempDir()}
	got, content := fetch(t, f, url, "")
	if len(got.Ref) != 12 || content != "head" {
		t.Errorf("head: %+v %q", got, content)
	}
	if _, err := f.Fetch(context.Background(), Spec{Location: url, Version: "1"}); err == nil {
		t.Error("a version was found in a repository without releases")
	}
}

func TestFetchLocal(t *testing.T) {
	dir := t.TempDir()
	got, err := (&Fetcher{}).Fetch(context.Background(), Spec{Local: true, Location: dir})
	if err != nil || got.Dir != dir || got.Ref != "" {
		t.Errorf("local: %+v %v", got, err)
	}
	if _, err := (&Fetcher{}).Fetch(context.Background(), Spec{Local: true, Location: filepath.Join(dir, "missing")}); err == nil {
		t.Error("missing local stack was accepted")
	}
}
