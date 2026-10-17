package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/conf"
	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/render"
	"github.com/quanarch/quanarch/internal/source"
	"github.com/quanarch/quanarch/internal/stack"
)

type newOptions struct {
	stack string
	dir   string
	quas  []string
	noRun bool
}

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <kind> <name>",
		Short: "Create a new quan, quanar or qua from a stack template",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	for _, kind := range quanarch.Kinds {
		cmd.AddCommand(newKindCmd(kind))
	}
	return cmd
}

func newKindCmd(kind quanarch.Kind) *cobra.Command {
	var opts newOptions
	cmd := &cobra.Command{
		Use:   string(kind) + " <name>",
		Short: fmt.Sprintf("Create a new %s, %s", kind, kind.Summary()),
		Example: fmt.Sprintf("  qu new %s %s --stack react-ts\n  qu new %s %s --stack github.com/acme/stack@1",
			kind, exampleName(kind), kind, exampleName(kind)),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, kind, args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.stack, "stack", "s", "", "stack to create from: an official name such as react-ts[@1], a git repository or a local path")
	cmd.Flags().StringVarP(&opts.dir, "dir", "d", "", "directory to create the project in (default ./<name>)")
	cmd.Flags().BoolVar(&opts.noRun, "no-run", false, "do not run the stack's setup commands")
	if kind != quanarch.Qua {
		cmd.Flags().StringArrayVarP(&opts.quas, "qua", "q", nil, "qua to wire in, as name[@version] or a local path such as ../qua-a (repeatable)")
	}
	return cmd
}

func exampleName(kind quanarch.Kind) string {
	switch kind {
	case quanarch.Quanar:
		return "main"
	case quanarch.Qua:
		return "qua-a"
	}
	return "billing"
}

func runNew(cmd *cobra.Command, kind quanarch.Kind, name string, opts newOptions) error {
	out := cmd.OutOrStdout()
	if err := quanarch.ValidateName(name); err != nil {
		return err
	}
	if opts.stack == "" {
		return fmt.Errorf("--stack is required (official stacks: %s)", strings.Join(source.Official(), ", "))
	}

	spec, err := source.ParseSpec(opts.stack)
	if err != nil {
		return err
	}
	cacheDir, err := source.DefaultCacheDir()
	if err != nil {
		return err
	}
	fetcher := &source.Fetcher{CacheDir: cacheDir}
	fetched, err := fetcher.Fetch(cmd.Context(), spec)
	if err != nil {
		return err
	}

	manifest, err := stack.LoadManifest(fetched.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s is not a stack: no %s found", spec, stack.ManifestFile)
	}
	if err != nil {
		return err
	}
	tmpl, ok := manifest.Templates[kind]
	if !ok {
		return fmt.Errorf("stack %s has no template for %s", manifest.Name, kind)
	}

	dst := opts.dir
	if dst == "" {
		dst = name
	}
	quas, err := resolveQuas(dst, opts.quas)
	if err != nil {
		return err
	}
	stackVersion := manifest.Version
	if stackVersion == "" {
		stackVersion = strings.TrimPrefix(fetched.Ref, "v")
	}
	data := render.Data{Name: name, Kind: string(kind), Stack: manifest.Name, StackVersion: stackVersion, Quas: quas}
	if err := render.Render(os.DirFS(filepath.Join(fetched.Dir, filepath.FromSlash(tmpl.Dir))), dst, data); err != nil {
		return err
	}

	// A broken conf.qu is the stack's fault, not the user's: warn and carry on.
	if _, err := conf.Load(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: the template produced an invalid %s: %v\n", conf.FileName, err)
	}

	if !opts.noRun {
		for _, args := range tmpl.Run {
			if err := runStep(cmd, dst, args); err != nil {
				return fmt.Errorf("created %s in %s, but %q failed: %w", kind, dst, strings.Join(args, " "), err)
			}
		}
	}

	return printCreated(out, kind, name, dst, manifest, stackVersion)
}

// resolveQuas turns --qua values into template data. A local path is read
// through its conf.qu, so the qua's own name and version are used.
func resolveQuas(dst string, values []string) ([]render.Qua, error) {
	var quas []render.Qua
	seen := map[string]bool{}
	for _, v := range values {
		var q render.Qua
		if source.IsLocalPath(v) {
			p, err := conf.Load(v)
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("--qua %s: no %s found", v, conf.FileName)
			}
			if err != nil {
				return nil, fmt.Errorf("--qua %s: %w", v, err)
			}
			if p.Kind != quanarch.Qua {
				return nil, fmt.Errorf("--qua %s: %s is a %s, not a qua", v, p.Name, p.Kind)
			}
			rel, err := relPath(dst, v)
			if err != nil {
				return nil, err
			}
			q = render.Qua{Name: p.Name, Version: p.Version, Path: rel}
		} else {
			ref, err := conf.ParseRef(v)
			if err != nil {
				return nil, fmt.Errorf("--qua %s: %w", v, err)
			}
			q = render.Qua{Name: ref.Name, Version: ref.Version}
		}
		key := q.Name + "@" + q.Version
		if seen[key] {
			return nil, fmt.Errorf("--qua %s is given more than once", key)
		}
		seen[key] = true
		quas = append(quas, q)
	}
	return quas, nil
}

// relPath returns the slash-separated path from directory from to target.
func relPath(from, target string) (string, error) {
	absFrom, err := filepath.Abs(from)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absFrom, absTarget)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func runStep(cmd *cobra.Command, dir string, args []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "→ %s\n", strings.Join(args, " "))
	step := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
	step.Dir = dir
	step.Stdin = os.Stdin
	step.Stdout = cmd.OutOrStdout()
	step.Stderr = cmd.ErrOrStderr()
	return step.Run()
}

func printCreated(w io.Writer, kind quanarch.Kind, name, dir string, m *stack.Manifest, version string) error {
	from := m.Name
	if version != "" {
		from += " " + version
	}
	_, err := fmt.Fprintf(w, "Created %s %s in %s from %s\n", kind, name, dir, from)
	return err
}
