package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quanarch/quanarch/internal/conf"
	createpkg "github.com/quanarch/quanarch/internal/create"
	"github.com/quanarch/quanarch/internal/project"
	"github.com/quanarch/quanarch/internal/quanarch"
	"github.com/quanarch/quanarch/internal/source"
	"github.com/quanarch/quanarch/internal/stack"
	"github.com/quanarch/quanarch/internal/version"
	"github.com/quanarch/quanarch/internal/wizard"
)

// newFlags are every answer the wizard can be given on the command line.
type newFlags struct {
	lang, dir, qua, newQua, quanar, quan, example string
}

// kindHelp is the help of `qu new <kind>`.
type kindHelp struct {
	kind    quanarch.Kind
	short   string
	long    string
	example string
}

var kindHelps = []kindHelp{
	{
		kind:  quanarch.Project,
		short: "Create a qua, a quanar and a quan together, already wired",
		long: `Creates a project folder with three parts that already talk to each other:

  <name>/
  ├── packages/<qua>/   the qua: the contract the quanar implements and the quan calls
  ├── quanar/           the quanar: loads quans and plugs the qua in
  └── quans/<quan>/     the first quan

Run the quanar and the quan as the project's README says, and the quan's
counter counts through the quanar.`,
		example: `  qu new project
  qu new project acme
  qu new project acme --lang react-ts --dir . --qua acme-qua --quanar acme-quanar --quan billing`,
	},
	{
		kind:  quanarch.Quanar,
		short: "Create a quanar: the host that loads quans and plugs in their quas",
		long: `Creates a quanar. It holds no business logic: it loads quans, mounts their
routes inside its layout and implements the quas they call.

Inside a project root it goes to quanar/ and lists the project's quans.`,
		example: `  qu new quanar main
  qu new quanar main --lang react-ts --dir . --qua acme-qua
  qu new quanar main --lang react-ts --dir ./apps --qua none`,
	},
	{
		kind:  quanarch.Quan,
		short: "Create a quan: business logic that lives inside a quanar",
		long: `Creates a quan: a manifest (name, version, routes, permissions) and its
folders. It cannot run on its own; a quanar loads it.

Inside a project root it goes to quans/, takes the next free port, is added to
the quanar's and the qmock's quan list, and its qua is plugged into them.`,
		example: `  qu new quan billing
  qu new quan billing --lang react-ts --dir . --qua acme-qua
  qu new quan billing --lang react-ts --dir . --new-qua billing-qua
  qu new quan billing --lang react-ts --dir ./out --qua acme-qua@^1.0.0`,
	},
	{
		kind:  quanarch.Qua,
		short: "Create a qua: the contract between quans and a quanar",
		long: `Creates a qua: the interface the quanar implements, init() to plug it in, and
the calls quans make. It holds no behaviour.

Inside a project root it goes to packages/.`,
		example: `  qu new qua acme-qua
  qu new qua acme-qua --lang react-ts --dir .`,
	},
	{
		kind:  quanarch.Qmock,
		short: "Create a qmock: a stand-in quanar for developing quans",
		long: `Creates a qmock: a quanar marked as a stand-in, on its own port, whose core
answers the quas with stand-in behaviour.

Inside a project root it goes to qmock/ and lists the project's quans.`,
		example: `  qu new qmock dev
  qu new qmock dev --lang react-ts --dir . --qua acme-qua`,
	},
	{
		kind:  wizard.Examples,
		short: "Create a complete example application",
		long: `Creates a complete application to learn from or start with.

React · TypeScript, advanced: a Node.js server with a JSON database, an
artifact store, a quanar with sign-in, a qua and four quans, each loaded a
different way (API, SSE, the quanar's public folder, the artifact store).

Node.js · TypeScript, advanced: a full-stack shop. A quanar server whose
managers turn what quans declare into tables, routes, permission checks and
validation; users, products and orders quans that talk by events, each in its
own process; and a React client.

Python, advanced: the same shop in Python. A Starlette quanar with the same
managers, quans that declare themselves with dataclasses, each in its own
Python process, and the React client.`,
		example: `  qu new examples
  qu new examples shop --lang react-ts --example advanced --dir .
  qu new examples shop --lang node-ts --example advanced --dir .
  qu new examples shop --lang python --example advanced --dir .`,
	},
}

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a project, quanar, quan, qua, qmock or example",
		Long: `Creates something new. Run it alone to be asked step by step, or name what to
create and answer the rest with flags:

  project    a qua, a quanar and a quan together, already wired
  quanar     the host that loads quans and plugs in their quas
  quan       business logic that lives inside a quanar
  qua        the contract between quans and a quanar
  qmock      a stand-in quanar for developing quans
  examples   complete applications

Whatever the command leaves out is asked; when it leaves nothing out, no
question is asked at all. Run "qu new <kind> --help" for a kind's flags.`,
		Example: "  qu new\n  qu new quan\n  qu new quan billing --lang react-ts --dir . --qua acme-qua",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runNew(cmd, wizard.Preset{})
		},
	}
	for _, h := range kindHelps {
		cmd.AddCommand(newKindCmd(h))
	}
	return cmd
}

func newKindCmd(h kindHelp) *cobra.Command {
	var f newFlags
	cmd := &cobra.Command{
		Use:     string(h.kind) + " [name]",
		Short:   h.short,
		Long:    h.long,
		Example: h.example,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			preset, err := presetFrom(h.kind, args, f)
			if err != nil {
				return err
			}
			return runNew(cmd, preset)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&f.lang, "lang", "", "language: "+languageIDs())
	flags.StringVar(&f.dir, "dir", "", "where to create it; a project root puts each kind in its place")
	switch h.kind {
	case quanarch.Project:
		flags.StringVar(&f.qua, "qua", "", "name of the project's qua")
		flags.StringVar(&f.quanar, "quanar", "", "name of the project's quanar")
		flags.StringVar(&f.quan, "quan", "", "name of the project's first quan")
	case quanarch.Quanar, quanarch.Quan, quanarch.Qmock:
		flags.StringVar(&f.qua, "qua", "", "qua to use: a name in the project's packages/, a path, name@version or name@url, or none")
		flags.StringVar(&f.newQua, "new-qua", "", "create a new qua with this name in the project's packages/ and use it")
	case wizard.Examples:
		flags.StringVar(&f.example, "example", "", "example to create, for example advanced")
	}
	return cmd
}

func languageIDs() string {
	ids := make([]string, 0, len(wizard.Languages))
	for _, l := range wizard.Languages {
		if l.Available {
			ids = append(ids, l.ID)
		}
	}
	return strings.Join(ids, ", ")
}

// presetFrom turns a kind's arguments and flags into wizard answers.
func presetFrom(kind quanarch.Kind, args []string, f newFlags) (wizard.Preset, error) {
	p := wizard.Preset{Kind: kind}
	cwd, err := os.Getwd()
	if err != nil {
		return p, err
	}
	if len(args) == 1 {
		if err := quanarch.ValidateName(args[0]); err != nil {
			return p, err
		}
		p.Name = args[0]
	}
	if f.lang != "" {
		lang, err := findLanguage(f.lang)
		if err != nil {
			return p, err
		}
		p.Language = &lang
	}
	if f.dir != "" {
		if p.Dir, err = wizard.ResolveDir(cwd, f.dir); err != nil {
			return p, fmt.Errorf("--dir: %w", err)
		}
	}
	switch kind {
	case quanarch.Project:
		p.QuaName, p.QuanarName, p.QuanName = f.qua, f.quanar, f.quan
	case quanarch.Quanar, quanarch.Quan, quanarch.Qmock:
		where := p.Dir
		if where == "" {
			where = cwd
		}
		if p.Qua, err = quaFrom(f, where); err != nil {
			return p, err
		}
	case wizard.Examples:
		p.Example = f.example
	}
	return p, nil
}

func findLanguage(id string) (wizard.Language, error) {
	for _, l := range wizard.Languages {
		if l.ID == id {
			return l, nil
		}
	}
	return wizard.Language{}, fmt.Errorf("--lang: unknown language %q (available: %s)", id, languageIDs())
}

// quaFrom reads --qua and --new-qua; nil means neither was given.
func quaFrom(f newFlags, dir string) (*createpkg.QuaSource, error) {
	switch {
	case f.newQua != "" && f.qua != "":
		return nil, fmt.Errorf("give either --qua or --new-qua, not both")
	case f.newQua != "":
		if err := quanarch.ValidateName(f.newQua); err != nil {
			return nil, fmt.Errorf("--new-qua: %w", err)
		}
		return &createpkg.QuaSource{From: createpkg.NewQua, Name: f.newQua}, nil
	case f.qua == "":
		return nil, nil
	case f.qua == "none":
		return &createpkg.QuaSource{From: createpkg.NoQua}, nil
	case strings.Contains(f.qua, "@"):
		name, spec, err := wizard.ParseQuaSpec(f.qua)
		if err != nil {
			return nil, fmt.Errorf("--qua: %w", err)
		}
		return &createpkg.QuaSource{From: createpkg.PublishedQua, Name: name, Spec: spec}, nil
	case source.IsLocalPath(f.qua):
		abs, err := filepath.Abs(f.qua)
		if err != nil {
			return nil, err
		}
		p, err := conf.Load(abs)
		if err != nil || p.Kind != quanarch.Qua {
			return nil, fmt.Errorf("--qua %s: not a qua (no conf.qu declaring one)", f.qua)
		}
		return &createpkg.QuaSource{From: createpkg.LocalQua, Name: p.Name, Dir: abs}, nil
	}
	if root, ok := project.FindRoot(dir); ok {
		for _, q := range project.Quas(root) {
			if q.Name == f.qua {
				return &createpkg.QuaSource{From: createpkg.LocalQua, Name: q.Name, Dir: q.Dir}, nil
			}
		}
	}
	return nil, fmt.Errorf("--qua %s: no qua with that name in the project's packages/; use name@version for a published one, a path, or none", f.qua)
}

// runNew creates at once when the command answered everything, and asks for the rest otherwise.
func runNew(cmd *cobra.Command, preset wizard.Preset) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ver := version.Label()

	if m := wizard.New(preset, cwd, ver); m.Complete() {
		a := m.Answers()
		if err := wizard.Validate(a); err != nil {
			return err
		}
		return create(cmd, a)
	} else if !wizard.IsInteractive() {
		return fmt.Errorf("missing %s; give them on the command line, or run in a terminal to be asked", strings.Join(m.Missing(), ", "))
	}

	answers, err := wizard.Run(preset, ver, cmd.OutOrStdout())
	if errors.Is(err, wizard.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}
	return create(cmd, answers)
}

func create(cmd *cobra.Command, a wizard.Answers) error {
	out := cmd.OutOrStdout()
	st, err := resolveStack(cmd, a.Language)
	if err != nil {
		return err
	}
	if a.Kind == wizard.Examples {
		if err := createpkg.NewExample(a.Example.ID, a.Name, a.Target(), st, true, out); err != nil {
			return err
		}
		fmt.Fprintf(out, "\n Done.\n")
		if start := st.Manifest.Examples[a.Example.ID].Start; start != nil {
			fmt.Fprintf(out, " Start everything with:\n\n   cd %s && %s\n", a.Name, strings.Join(start, " "))
		}
		fmt.Fprintln(out)
		return nil
	}
	if a.Kind != quanarch.Project {
		return createSingle(cmd, a, st)
	}
	p := createpkg.Project{Dir: a.Dir, Name: a.Name, Qua: a.Qua, Quanar: a.Quanar, Quan: a.Quan}
	if err := createpkg.NewProject(p, st, true, out); err != nil {
		return err
	}

	quanStart, quanarStart := st.Start(quanarch.Quan), st.Start(quanarch.Quanar)
	fmt.Fprintf(out, "\n Done.\n")
	if quanStart != nil && quanarStart != nil {
		fmt.Fprintf(out, " Run it in two terminals:\n\n")
		fmt.Fprintf(out, "   cd %s && %s\n", filepath.Join(a.Name, "quanar"), strings.Join(quanarStart, " "))
		fmt.Fprintf(out, "   cd %s && %s\n", filepath.Join(a.Name, "quans", a.Quan), strings.Join(quanStart, " "))
		if port := st.Port(quanarch.Quanar); port != 0 {
			fmt.Fprintf(out, "\n Then open http://localhost:%d/%s\n", port, a.Quan)
		}
	}
	fmt.Fprintln(out)
	return nil
}

func createSingle(cmd *cobra.Command, a wizard.Answers, st createpkg.Stack) error {
	out := cmd.OutOrStdout()
	target := a.Target()
	res, err := createpkg.NewSingle(createpkg.Single{Kind: a.Kind, Name: a.Name, Target: target, Qua: a.QuaSource}, st, true, out)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n Done.\n\n")
	start := st.Start(a.Kind)
	switch a.Kind {
	case quanarch.Quan:
		if start != nil {
			fmt.Fprintf(out, "   cd %s && %s    # serves the quan on port %d\n\n", target, strings.Join(start, " "), res.Port)
		}
		for _, host := range res.Wired {
			fmt.Fprintf(out, " Plugged its qua into %s\n", host)
		}
		for _, host := range res.Registered {
			fmt.Fprintf(out, " Added to %s\n", host)
		}
		if len(res.Registered) == 0 && len(res.Notes) == 0 {
			fmt.Fprintf(out, " Add it to your quanar's quan list: %s, served on port %d.\n", a.Name, res.Port)
		}
	case quanarch.Quanar, quanarch.Qmock:
		if start != nil {
			comment := ""
			if res.Port != 0 {
				comment = fmt.Sprintf("    # http://localhost:%d", res.Port)
			}
			fmt.Fprintf(out, "   cd %s && %s%s\n", target, strings.Join(start, " "), comment)
		}
	case quanarch.Qua:
		fmt.Fprintf(out, " Link %s from a quanar and its quans.\n", target)
	}
	for _, note := range res.Notes {
		fmt.Fprintf(out, "\n ! %s\n", note)
	}
	if needsQua := a.Kind != quanarch.Qua; needsQua && a.QuaSource.From == createpkg.NoQua {
		fmt.Fprintf(out, "\n It has no qua yet: it cannot talk to a quanar until one is added. See its README.\n")
	}
	fmt.Fprintln(out)
	return nil
}

// resolveStack finds the templates for a language. QU_STACK_DIR points at a
// stack on disk instead, which is how stacks are developed.
func resolveStack(cmd *cobra.Command, lang wizard.Language) (createpkg.Stack, error) {
	dir := os.Getenv("QU_STACK_DIR")
	if dir == "" {
		spec, err := source.ParseSpec(lang.ID)
		if err != nil {
			return createpkg.Stack{}, err
		}
		cacheDir, err := source.DefaultCacheDir()
		if err != nil {
			return createpkg.Stack{}, err
		}
		fetched, err := (&source.Fetcher{CacheDir: cacheDir}).Fetch(cmd.Context(), spec)
		if err != nil {
			return createpkg.Stack{}, err
		}
		dir = fetched.Dir
	}
	m, err := stack.LoadManifest(dir)
	if err != nil {
		return createpkg.Stack{}, err
	}
	return createpkg.Stack{Dir: dir, Manifest: m}, nil
}
