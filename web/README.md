# web/

Front-end sites for this project. One subdirectory per site:

```
web/
├── <site-name>/         # Astro site (or other static-first framework)
│   ├── package.json
│   ├── astro.config.mjs
│   ├── src/
│   └── public/
└── README.md
```

## Convention

- **One site per subdirectory.** Examples: `web/marketing/`, `web/docs/`, `web/app/`.
- **Astro is the default** for static-first sites. Other frameworks (Next, SvelteKit) are allowed when the requirements demand it; justify the choice in an ADR.
- **Site name is kebab-case.** Match the deployed hostname or purpose, not the framework.
- Each site is self-contained: its own `package.json`, `node_modules/`, build output, and CI hooks.

## Creating a new site

```bash
cd web
npm create astro@latest <site-name>
cd <site-name>
npm install
```

Then commit the resulting scaffold.

## Standards

All sites under `web/` follow `~/.ai/Code.md §10` (accessibility, performance,
responsive design, SEO, privacy). In particular:

- WCAG 2.1 AA minimum
- Core Web Vitals: LCP < 2.5s, INP < 200ms, CLS < 0.1 on mid-range mobile / slow 4G
- Mobile-first, progressive enhancement
- Open Graph + Twitter Card metadata on shareable pages
- Cookie banners honor user choices, not just display them
