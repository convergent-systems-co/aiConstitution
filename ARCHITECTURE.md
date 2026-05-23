# Architecture

(Replace this stub. See `docs/adr/` for individual decisions.)

## Repository layout

```
src/                    Go source — all of it
  cmd/<app>/            entry points (one per binary)
  internal/             internal packages (not importable by other modules)
  pkg/                  public packages
  plugins/              Go-loadable plugins
web/                    Front-end sites
  <site-name>/          one subdirectory per Astro (or equivalent) site
docs/                   Project documentation
  adr/                  MADR-format architecture decisions
scripts/                Project tooling
```

## High-level

(Describe the major components and how they communicate.)

## Diagram

(Mermaid or PlantUML. Source-not-binary per Code.md §9.2.)
