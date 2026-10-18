# stack.qu

A **stack** is a set of templates for one language or framework, such as `react-ts`.
`qu` itself knows nothing about languages; everything language-specific lives in
stacks. A stack is a directory, usually a git repository, with a `stack.qu` file at
its root.

`stack.qu` uses the same syntax as [`conf.qu`](conf-qu.md).

```
stack react-ts
version 0.3.1
title "React · TypeScript"

template project templates/project
template qua templates/qua {
	run npm install
	run npm run build
}
template quanar templates/quanar {
	run npm install
	port 5173
	start npm run dev
}
template quan templates/quan {
	run npm install
	port 4001
	start npm run watch
}
template qmock templates/quanar {
	run npm install
	port 5174
	start npm run dev
}

example advanced examples/advanced {
	start make run
}

on quan insert src/config/index.ts "];" "  { name: '[[ .Name ]]', url: 'http://localhost:[[ .Port ]]/[[ .Name ]].js' },"
on qua insert src/main.tsx "// qu:qua-imports" "import * as [[ pascal .Name ]] from '[[ .Name ]]';"
on qua insert src/core/qua-core/index.ts "// qu:qua-cores" {
	line "const [[ camel .Name ]]Core: [[ pascal .Name ]].QuanarCore = {"
	line "  increment: (value) => value + 1,"
	line "};"
}
on qua run npm install [[ .Dependency ]]
```

## Directives

| Directive | Description |
| --- | --- |
| `stack <name>` | First line. The stack's name, the ID of the language chosen in `qu` (`react-ts`). |
| `version <version>` | The stack's version. |
| `title <text>` | A human-readable name shown in prompts. |
| `template <kind> <dir> [{ ... }]` | The template directory for `project`, `quanar`, `quan`, `qua` or `qmock`, relative to the stack root. Two kinds may share a directory; templates can tell them apart with `.Kind`. |
| `example <id> <dir> [{ ... }]` | A complete example application, offered under *examples* in `qu`. The directory is rendered as a whole into the new project folder. |
| `run <command> [args...]` | Inside a `template` or `example` block. A command run in the new project after it is created. Commands run in order, without a shell. |
| `port <number>` | Inside a `template` block. The port the part is served on while it is developed. For a quan it is the port of a project's first quan; the next ones count up from it. |
| `start <command> [args...]` | Inside a `template` or `example` block. The command that runs the part while it is developed; `qu` shows it when it is done. |
| `on quan …`<br>`on qua …` | How a quanar or qmock takes in a quan added to the project later, or a qua such a quan brings. See below. |

## Growing a project

`qu` knows no language, so it cannot know where a quanar lists its quans or how it
plugs in a qua. The stack says it with `on` directives. When a quan is added to a
project, `qu` goes through the project's quanar and qmock and, in each one:

1. for every qua of the quan that the host's `conf.qu` does not list yet, does what
   the `on qua` directives say, then adds a `qua` line to the host's `conf.qu`;
2. does what the `on quan` directives say.

```
on <quan|qua> insert <file> <marker> <line>
on <quan|qua> insert <file> <marker> {
	line <line>
	line <line>
}
on <quan|qua> run <command> [args...]
```

- `insert` puts the lines just above the line of `file` that holds exactly `marker`,
  indented like it. `file` is relative to the quanar or qmock. Every file and marker
  is checked before anything is written; if one is missing nothing changes and `qu`
  says what to do by hand. Lines already in place are not inserted again.
- `run` runs a command in the quanar or qmock, for example to add the qua as a
  dependency with the language's own package manager.
- Lines and commands are templates. An `on quan` directive sees the new quan: `.Name`,
  `.Port` and `.Quas`, its quas, whose `.Dependency` is relative to the host. An
  `on qua` directive sees the qua: `.Name`, `.Version` and `.Dependency`. A command is
  rendered as one line before it is split into words, so `[[ .Dependency ]]` needs no quotes.

A template that leaves markers in its files, such as `// qu:qua-imports`, lets its
stack grow a project without touching anything else.

## Templates

A template directory is copied into the new project as it is, with two exceptions:

- Files ending in `.qtpl` are rendered as [Go templates](https://pkg.go.dev/text/template)
  and the suffix is dropped: `conf.qu.qtpl` becomes `conf.qu`.
- Path segments may contain template actions: `src/[[ pascal .Name ]].tsx`.

Templates use `[[` and `]]` as delimiters, so the `{{ }}` of Vue, Angular, Handlebars
or Go needs no escaping. A `.git` directory is never copied.

| Value | Example |
| --- | --- |
| `.Name` | `billing-report` |
| `.Kind` | `project`, `quanar`, `quan`, `qua` or `qmock` |
| `.Stack` | `react-ts` |
| `.StackVersion` | `0.3.0` |
| `.Project` | The name of the project the new item belongs to, or empty. |
| `.Port` | The port it is served on during development, from the stack's `port`: for a quan the next free one (`4001`, `4002`…), for a quanar or qmock its own (`5173`, `5174`). |
| `.Quas` | The quas it is wired to. Each has `.Name` (`acme-qua`), `.Version` (`1`, may be empty), `.Path` (`../packages/acme-qua` for a qua on disk, otherwise empty) and `.Dependency`, what a package manager needs to fetch it (`file:../packages/acme-qua`, `^1.0.0` or a URL). Empty when no qua was chosen. |
| `.Quans` | For a project, quanar or qmock: the quans it loads. Each has `.Name` and `.Port`. |

| Function | `billing-report` becomes |
| --- | --- |
| `pascal` | `BillingReport` |
| `camel` | `billingReport` |
| `snake` | `billing_report` |
| `kebab` | `billing-report` |
| `title` | `Billing Report` |
| `upper`, `lower` | `BILLING-REPORT`, `billing-report` |

A template should include a `conf.qu.qtpl` so the new project can describe itself:

```
[[ .Kind ]] [[ .Name ]]
stack [[ .Stack ]]@[[ .StackVersion ]]
[[ if .Port ]]port [[ .Port ]]
[[ end ]][[ range .Quas ]]
qua [[ .Name ]][[ if .Version ]]@[[ .Version ]][[ end ]]
[[- end ]]
```

## Developing a stack

`qu` fetches a stack from its repository's latest release tag. To use a stack on disk
instead, point `QU_STACK_DIR` at it:

```sh
QU_STACK_DIR=../quanarch-node qu new project acme --lang node-ts --dir . --qua acme-qua --quanar acme-quanar --quan billing
```

With `QU_STACK_DIR` set, a language whose stack is not released yet can be chosen too.
