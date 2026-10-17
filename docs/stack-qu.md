# stack.qu

A **stack** is a set of templates for one language or framework, such as `react-ts`.
`qu` itself knows nothing about languages; everything language-specific lives in
stacks. A stack is a directory, usually a git repository, with a `stack.qu` file at
its root.

`stack.qu` uses the same syntax as [`conf.qu`](conf-qu.md).

```
stack react-ts
version 1.0.0
title "React + TypeScript"

template quan templates/quan {
	run npm install
}
template quanar templates/quanar
template qua templates/qua
```

## Directives

| Directive | Description |
| --- | --- |
| `stack <name>` | First line. The stack's name, used as `--stack <name>`. |
| `version <version>` | The stack's version. |
| `title <text>` | A human-readable name shown in prompts. |
| `template <kind> <dir> [{ ... }]` | The template directory for `quan`, `quanar` or `qua`, relative to the stack root. A stack may provide any subset of the three kinds. |
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
| `.Kind` | `quan` |
| `.Stack` | `react-ts` |
| `.StackVersion` | `1.0.0` |

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
```
