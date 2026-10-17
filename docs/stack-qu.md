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
