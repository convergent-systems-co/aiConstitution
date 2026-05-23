# aiConstitution

Go CLI for the AI governance system (Constitution / Common / Code / Writing).

> Scaffolded from
> [`convergent-systems-co/go-app-template`](https://github.com/convergent-systems-co/go-app-template).

## Layout

```
src/
  cmd/ai/          entry point (binary: ai)
  internal/        internal packages
  pkg/             public packages
  plugins/         Go-loadable plugins
web/               front-end sites (Astro default)
docs/adr/          architecture decisions (MADR)
scripts/           project tooling
```

## Build

```bash
go work sync
make build         # produces dist/ai
./dist/ai
```

## Test / lint

```bash
make test
make lint
```

## License

AGPL-3.0. See `LICENSE` and `COPYRIGHT`.
