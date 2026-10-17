package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var errNoReleases = errors.New("no releases")

// Fetched is a stack available on disk.
type Fetched struct {
	Dir string
	// Ref is the release tag, or the commit for a repository without releases.
	// It is empty for a local stack.
	Ref string
}

// Fetcher downloads stacks with git and keeps them in a cache.
type Fetcher struct {
	// CacheDir is the root of the cache; stacks are kept under CacheDir/stacks.
	CacheDir string
	// Git is the git executable. Defaults to "git".
	Git string
}

// DefaultCacheDir returns the user's cache directory for quanarch.
func DefaultCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "quanarch"), nil
}

// Fetch makes the stack described by spec available on disk.
//
// A git stack resolves to the highest release tag that matches spec.Version.
// A repository without release tags resolves to its default branch, but only
// when no version was asked for. Releases are cached and never fetched twice.
func (f *Fetcher) Fetch(ctx context.Context, spec Spec) (Fetched, error) {
	if spec.Local {
		info, err := os.Stat(spec.Location)
		if err != nil {
			return Fetched{}, err
		}
		if !info.IsDir() {
			return Fetched{}, fmt.Errorf("%s is not a directory", spec.Location)
		}
		return Fetched{Dir: spec.Location}, nil
	}

	tags, err := f.tags(ctx, spec.Location)
	if err != nil {
		return Fetched{}, err
	}
	branch := ""
	ref, err := selectTag(tags, spec.Version)
	switch {
	case err == nil:
		branch = ref
	case errors.Is(err, errNoReleases):
		if ref, err = f.head(ctx, spec.Location); err != nil {
			return Fetched{}, err
		}
	default:
		return Fetched{}, fmt.Errorf("%s: %w", spec.Location, err)
	}

	dir := filepath.Join(f.CacheDir, "stacks", cacheName(spec.Location), ref)
	if _, err := os.Stat(dir); err == nil {
		return Fetched{Dir: dir, Ref: ref}, nil
	}
	if err := f.clone(ctx, spec.Location, branch, dir); err != nil {
		return Fetched{}, err
	}
	return Fetched{Dir: dir, Ref: ref}, nil
}

func (f *Fetcher) tags(ctx context.Context, url string) ([]string, error) {
	out, err := f.git(ctx, "ls-remote", "--tags", "--refs", url)
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, line := range strings.Split(out, "\n") {
		if _, ref, ok := strings.Cut(line, "\t"); ok {
			tags = append(tags, strings.TrimPrefix(ref, "refs/tags/"))
		}
	}
	return tags, nil
}

func (f *Fetcher) head(ctx context.Context, url string) (string, error) {
	out, err := f.git(ctx, "ls-remote", url, "HEAD")
	if err != nil {
		return "", err
	}
	sha, _, _ := strings.Cut(strings.TrimSpace(out), "\t")
	if len(sha) < 12 {
		return "", fmt.Errorf("%s: repository is empty", url)
	}
	return sha[:12], nil
}

// clone shallow-clones url into dir through a temporary directory, so an
// interrupted clone never leaves a half-filled cache entry behind.
func (f *Fetcher) clone(ctx context.Context, url, branch, dir string) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".clone-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	args := []string{"clone", "--quiet", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, "--", url, tmp)
	if _, err := f.git(ctx, args...); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(tmp, ".git")); err != nil {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another process may have filled the cache first.
		if _, statErr := os.Stat(dir); statErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func (f *Fetcher) git(ctx context.Context, args ...string) (string, error) {
	bin := f.Git
	if bin == "" {
		bin = "git"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("git is required to fetch stacks but was not found in PATH")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return stdout.String(), nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// cacheName turns a repository URL into a single directory name.
func cacheName(url string) string {
	if _, rest, ok := strings.Cut(url, "://"); ok {
		url = rest
	}
	url = strings.TrimSuffix(strings.TrimPrefix(url, "git@"), ".git")
	return strings.Trim(unsafeChars.ReplaceAllString(url, "_"), "_.")
}
