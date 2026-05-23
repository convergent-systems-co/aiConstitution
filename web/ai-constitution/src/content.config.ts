import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';

// Load the canonical Markdown docs from the repo root. The glob is
// relative to the Astro project root (web/ai-constitution/), so
// `../../` resolves to the repo root.
//
// Pattern intentionally enumerates each file so we don't accidentally
// pick up a future top-level *.md we don't want to publish (e.g.
// HANDOFF.md if one ever lands at the root).
const docs = defineCollection({
  loader: glob({
    pattern: [
      'SPEC.md',
      'GOALS.md',
      'ARCHITECTURE.md',
      'README.md',
      'CHANGELOG.md',
    ],
    base: '../../',
  }),
});

export const collections = { docs };
