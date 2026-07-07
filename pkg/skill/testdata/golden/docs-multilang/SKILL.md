---
name: docs-fixture
description: Docs Fixture — A documentation fixture. Use when answering questions about Docs Fixture.
---

# Docs Fixture

A documentation fixture.

## How to use this skill

- Find the most relevant page in the contents below and read it before answering.
- For keyword lookups across all pages, grep the `references/` directory.
- Pages are verbatim copies of the site's Hugo source (section `_index.md` files also
  get a generated listing appended): they start with front matter metadata and may
  contain shortcodes like `{{< note >}}`; read through them.
- Links inside pages: relative links point at sibling files here; `{{< relref "x" >}}` /
  `{{< ref "x" >}}` and site-absolute links name a content page — find the matching file
  under `references/`, or browse https://docs.example.com/ + the path.
- The live URL of a page ≈ https://docs.example.com/ + its `references/`-relative path without `.md`
  (`_index.md` → the directory URL); use that when citing sources.
- Content was extracted from the site at generation time and may have drifted since.

## Contents

### [Docs Fixture](references/_index.md) — 8 pages

### [Documentation](references/docs/_index.md) — 7 pages

- [Getting started](references/docs/getting-started.md) — First steps with the fixture
- [Install](references/docs/install/index.md) — Installing the fixture
- [Never rendered](references/docs/never-render.md)

### [Concepts](references/docs/concepts/_index.md) — 3 pages

- [Volumes](references/docs/concepts/volumes.md) — How fixture volumes work
- [Snapshots](references/docs/concepts/snapshots.md) — Point-in-time copies of volumes

Anything not covered here: https://docs.example.com/
