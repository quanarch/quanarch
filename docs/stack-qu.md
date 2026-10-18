# stack.qu

A **stack** is a set of templates for one language or framework, such as `react-ts`.
`qu` itself knows nothing about languages; everything language-specific lives in
stacks. A stack is a directory, usually a git repository, with a `stack.qu` file at
its root.

`stack.qu` uses the same syntax as [`conf.qu`](conf-qu.md).

```
stack react-ts
version 0.3.0
title "React · TypeScript"

template project templates/project
template qua templates/qua {
	run npm install
	run npm run build
}
template quanar templates/quanar {
	run npm install
}
template quan templates/quan {
	run npm install
}
template qmock templates/quanar {
	run npm install
}

example advanced examples/advanced
```

## Directives

| Directive | Description |
| --- | --- |
| `stack <name>` | First line. The stack's name, the ID of the language chosen in `qu` (`react-ts`). |
| `version <version>` | The stack's version. |
| `title <text>` | A human-readable name shown in prompts. |
| `template <kind> <dir> [{ ... }]` | The template directory for `project`, `quanar`, `quan`, `qua` or `qmock`, relative to the stack root. Two kinds may share a directory; templates can tell them apart with `.Kind`. |
| `example <id> <dir> [{ ... }]` | A complete example application, offered under *examples* in `qu`. The directory is rendered as a whole into the new project folder. |
| `run <command> [args...]` | Inside a `template` block. A command run in the new project after it is created. Commands run in order, without a shell. |

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
| `.Port` | For a quan, the port it is served on during development (`4001`, `4002`…); for a quanar or qmock, its dev server port (`5173`, `5174`). |
| `.Quas` | The quas it is wired to. Each has `.Name` (`acme-qua`), `.Version` (`1`, may be empty), `.Path` (`../packages/acme-qua` for a qua on disk, otherwise empty) and `.Dependency`, the value for package.json (`file:../packages/acme-qua`, `^1.0.0` or a URL). Empty when no qua was chosen. |
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
[[ range .Quas ]]
qua [[ .Name ]][[ if .Version ]]@[[ .Version ]][[ end ]]
[[- end ]]
```
