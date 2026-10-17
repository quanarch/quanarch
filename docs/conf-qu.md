# conf.qu

`conf.qu` identifies a Quanarch project. It sits at the root of a quan, quanar or
qua and tells tools what the project is, which stack it was made from and which quas
it is wired to.

The file is optional. Quanarch is a philosophy, not a set of rules: a project without
`conf.qu` is still a quan, it just cannot be inspected by `qu`.

```
# Billing quan
quan billing
stack react-ts@1
version 0.3.0

qua qua-a@1

zone admin {
	qua qua-b@2
}
```

Run `qu check` in a project directory to validate the file.

## Syntax

- One directive per line: a name followed by arguments separated by whitespace.
- `#` starts a comment that runs to the end of the line.
- Arguments containing whitespace can be wrapped in double quotes. Inside quotes,
  `\"` and `\\` are escapes.
- A directive opens a block by ending its line with `{`. The block is closed by a
  line holding only `}`.

## Directives

| Directive | Allowed in | Description |
| --- | --- | --- |
| `quan <name>`<br>`quanar <name>`<br>`qua <name>` | first line | Declares what the project is. Required, exactly once. |
| `stack <stack>[@version]` | all | The stack the project was created from, for example `react-ts@1`. |
| `version <version>` | all | The project's own version. For a qua this is the version of the contract. |
| `qua <name>[@version]` | quan, quanar | In a quan, a qua it uses. In a quanar, a qua it plugs in. |
| `zone <name> { ... }` | quan | An isolated area inside a quan. Holds only `qua` directives. |

Names are lowercase kebab-case (`billing-report`). Versions take the form `1`, `1.2`
or `1.2.3`, optionally followed by a pre-release suffix such as `2.0.0-beta.1`.

The same qua may be listed with different versions, for example `qua-a@1` and
`qua-a@2` while a quan moves from one to the other.
