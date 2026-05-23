# ai-constitution.convergent-systems.co

Astro site for the public aiConstitution methodology / spec / installer.

Per [`SPEC.md §14.1`](../../SPEC.md#141-one-stack-astro-everywhere-v07) this
site is one of five Convergent Systems Astro properties, all of which
consume the `convergent-systems@1.0.0` brand atom from
`brand-atoms.com`.

## Status

Scaffolded for v0.8. Live brand-atoms fetch and the rich content
collection of the spec are **deferred to morning work** (see the
top-level `GOALS.md` "Out of scope for v0.8").

For v0.8 the brand tokens are inlined in `src/styles/brand.css` from
the canonical 1.0.0 values stated in `SPEC.md §14.4` so the site
renders in the correct identity without a network fetch at build
time.

## Develop

```bash
cd web/ai-constitution
npm install
npm run dev      # http://localhost:4321
npm run build    # → dist/
npm run preview
```

Requirements: Node 20+.

## Routes

| Route | Purpose |
|---|---|
| `/` | Landing — what aiConstitution is, what it ships, status |
| `/install` | brew / scoop / winget / from-source |
| `/spec` | Section index linking back to `SPEC.md` on GitHub |
| `/methodology` | What "AI Constitution" means; design rationale |
| `/community` | How to contribute hooks, findings, and atoms |

## Brand discipline

The 14 typed constraints from `SPEC.md §14.4` are enforced visually
in `src/styles/brand.css`:

- Heading weights locked to 700 or 800.
- Wordmark letter-spacing locked to 0.18em.
- Section-label letter-spacing locked to 0.32em uppercase.
- The mark fill is `solar-gold` only (`#F4C75E`).
- Mark treatments forbidden: stretched / rotated / recolored / drop-shadow / inverted-without-variant.
- WCAG 2.1 AA contrast (4.5:1 for body, 3:1 for primary action).
- `ember-orange` (`#FF8A3D`) is for warmth/ambient accent ONLY — never for error/danger states. Error states use `#FF5555` (a brand-compatible red).
- Heading-to-body size ratio ≥ 1.8.

When the live brand-atoms fetch lands, build-time validation against
the atom's typed constraint set replaces the comment-driven discipline
above.

## License

AGPL-3.0 (inherits from the repo root).
