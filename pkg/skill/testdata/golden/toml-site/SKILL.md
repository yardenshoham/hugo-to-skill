---
name: toml-fixture-en
description: TOML Fixture EN — A TOML-configured fixture. Use when answering questions about TOML Fixture EN.
---

# TOML Fixture EN

A TOML-configured fixture.

## How to use this skill

- Find the most relevant page in the contents below and read it before answering.
- For keyword lookups across all pages, grep the `references/` directory.
- Pages are verbatim copies of the site's Hugo source (section `_index.md` files also
  get a generated listing appended): they start with front matter metadata and may
  contain shortcodes like `{{< note >}}`; read through them.
- Links inside pages: relative links point at sibling files here; `{{< relref "x" >}}` /
  `{{< ref "x" >}}` and site-absolute links name a content page — find the matching file
  under `references/`, or browse https://toml.example.com/ + the path.
- The live URL of a page ≈ https://toml.example.com/ + its `references/`-relative path without `.md`
  (`_index.md` → the directory URL); use that when citing sources.
- Content was extracted from the site at generation time and may have drifted since.

## Contents

### [TOML Fixture EN](references/_index.md) — 7 pages

- [About](references/about.md) — What the TOML fixture is

### [Guides](references/guides/_index.md) — 5 pages

- [Advanced usage](references/guides/advanced.md) — Power-user features
- [First steps](references/guides/first-steps.md) — Getting going with the fixture
- [Alpha guide](references/guides/alpha.md)
- [Beta guide](references/guides/beta.md)

Anything not covered here: https://toml.example.com/
