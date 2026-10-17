<p align="center">
  <img src=".github/assets/logo.png" alt="Quanarch" width="120">
</p>

<h1 align="center">Quanarch</h1>

<p align="center">
  An architecture style between the monolith and microservices.<br>
  <a href="https://quanarch.org">quanarch.org</a>
</p>

Quanarch is an architecture style that sits between the monolith and microservices.
A system is built from three concepts:

- **quan** — a unit of business logic. It cannot run on its own.
- **quanar** — the host that gives quans a place to live. It holds no business logic.
- **qua** — a contract, the only window a quan has into the quanar.

Because a quan never carries its own runtime, the same quans can be folded into a
monolith, plugged into a shared quanar, or each given a quanar of its own — without
rewriting their logic.

Read the full philosophy at **[quanarch.org](https://quanarch.org)**.

This repository contains `quanarch`, the command-line tool (`qu` for short).

> [!NOTE]
> Quanarch is in early development. Commands and file formats may change before 1.0.

## Usage

```sh
qu new quan billing          # create a quan
qu new quanar main           # create a quanar
qu new qua qua-a             # create a qua

qu new quan billing --stack react-ts
qu version
```

The tool itself knows nothing about languages or frameworks. Templates come from
**stacks**, separate repositories such as `quanarch-react`, so any language can
join the ecosystem.

## Building from source

Requires Go 1.25 or later.

```sh
go build -o bin/qu ./cmd/qu
go test ./...
```

## License

Quanarch was created by [Can Berk Coşar](https://berkcosar.com) and is developed at
[Relteco](https://relteco.com).

Licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE).
