import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';

// Per SPEC.md §14.1 the canonical hostname is the kebab-case form;
// the camelCase form is a 301 alias.
export default defineConfig({
  site: 'https://ai-constitution.convergent-systems.co',
  integrations: [mdx(), sitemap()],
  trailingSlash: 'never',
  build: {
    assets: 'assets',
  },
  // TODO morning: integrate brand-atoms.com fetch (per SPEC.md §14.4).
  // For v0.8 the brand tokens are inlined in src/styles/brand.css from
  // the convergent-systems@1.0.0 atom values stated in SPEC.md §14.4.
});
