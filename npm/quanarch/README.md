# quanarch

`qu`, the command-line tool of [Quanarch](https://quanarch.org), as an npm package.

```sh
npm i -g quanarch --registry https://repo.relteco.com/repository/npm-relteco/
qu version
```

The package holds no code of its own: npm installs the one `@quanarch/qu-<os>-<cpu>`
package that matches the machine, and `qu` and `quanarch` run the binary inside it.
The binaries are the ones published on [GitHub Releases](https://github.com/quanarch/quanarch/releases).

Licensed under the Apache License, Version 2.0.
