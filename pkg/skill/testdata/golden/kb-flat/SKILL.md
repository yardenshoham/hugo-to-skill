---
name: fixture-kb
description: The Fixture Knowledge Base — Example knowledge base for testing. Use when answering questions about The Fixture Knowledge Base.
license: Apache-2.0
compatibility: Requires nothing
allowed-tools: Read Grep
metadata:
  category: kb
  source: fixture
---

# The Fixture Knowledge Base

Example knowledge base for testing.

## How to use this skill

- Find the most relevant page in the contents below and read it before answering.
- For keyword lookups across all pages, grep the `references/` directory.
- Pages are verbatim copies of the site's Hugo source (section `_index.md` files also
  get a generated listing appended): they start with front matter metadata and may
  contain shortcodes like `{{< note >}}`; read through them.
- Links inside pages: relative links point at sibling files here; `{{< relref "x" >}}` /
  `{{< ref "x" >}}` and site-absolute links name a content page — find the matching file
  under `references/`, or browse https://kb.example.com/ + the path.
- The live URL of a page ≈ https://kb.example.com/ + its `references/`-relative path without `.md`
  (`_index.md` → the directory URL); use that when citing sources.
- Content was extracted from the site at generation time and may have drifted since.

## Notes

- Fixture note.

## Contents

### [The Fixture Knowledge Base](references/kb/_index.md) — 5 pages

- [Best practices for node maintenance](references/kb/maintenance-and-upgrade.md) — Draining and upgrading fixture nodes
- [Empty](references/kb/empty.md)
- [Manually titled page](references/kb/no-front-matter.md)
- [Troubleshooting: volume detached](references/kb/troubleshooting-volume-detached.md) — What to do when a volume is stuck detached

Anything not covered here: https://kb.example.com/
