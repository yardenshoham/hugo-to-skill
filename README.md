# hugo-to-skill

hugo-to-skill is a utility to generate an [Agent Skills](https://agentskills.io)-compatible skill from a Hugo-based website, so an agent equipped with the skill knows the site's content.

# Usage

```bash
hugo-to-skill generate SITE_PATH_OR_GIT_URL --output SKILL_DIRECTORY
```

The site source may be a local directory or a git URL:

```bash
# Local clone, whole site
hugo-to-skill generate ./website --output ./skills/longhorn

# Straight from GitHub, scoped to one content section
hugo-to-skill generate https://github.com/longhorn/website --content-path kb --output ./skills/longhorn-kb

# Multilingual site, one language
hugo-to-skill generate https://github.com/kubernetes/website --lang en --content-path docs --output ./skills/kubernetes-docs
```

Content pages are copied into the skill's `references/` directory, mirroring the Hugo content tree. The generated `SKILL.md` and per-section `_index.md` listings provide the hierarchical index.

## Flags

| Flag               | Description                                                                                                              |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `--output`, `-o`   | **(required)** Output directory for the generated skill                                                                   |
| `--content-path`   | Restrict extraction to a subpath of the content dir (e.g. `kb`, `docs`), applied after language selection                 |
| `--lang`           | Language for multilingual sites (default: site's `defaultContentLanguage`)                                                |
| `--name`           | Override skill name (default: slugified site title, plus the `--content-path` when given)                                 |
| `--description`    | Override skill description (default: built from site title and `params.description`)                                      |
| `--license`        | License identifier (e.g., `Apache-2.0`)                                                                                   |
| `--compatibility`  | Compatibility requirements description                                                                                    |
| `--allowed-tools`  | Allowed tools specification                                                                                                |
| `--metadata`       | Metadata key=value pairs (can be repeated)                                                                                 |
| `--notes`          | Usage notes (can be repeated)                                                                                              |
| `--include-drafts` | Include pages Hugo would not publish (draft/future/expired)                                                                |

## Example output

Running `hugo-to-skill generate https://github.com/longhorn/website --content-path kb --output ./skills/longhorn-kb` produces:

```
skills/longhorn-kb/
├── SKILL.md
└── references/
    └── kb/
        ├── _index.md
        ├── troubleshooting-volume-readonly-or-io-error.md
        ├── space-consumption-guideline.md
        └── ... (62 pages, verbatim copies of the site's content files)
```

**SKILL.md:**

```markdown
---
name: longhorn-kb
description: "The Longhorn Knowledge Base — Cloud native distributed block storage for Kubernetes. Use when answering questions about The Longhorn Knowledge Base."
---

# The Longhorn Knowledge Base

Cloud native distributed block storage for Kubernetes.

## How to use this skill

- Find the most relevant page in the contents below and read it before answering.
- For keyword lookups across all pages, grep the `references/` directory.
- Pages are verbatim copies of the site's Hugo source: they start with front matter
  metadata and may contain shortcodes like `{{< note >}}`; read through them.
- Links inside pages: relative links point at sibling files here; `{{< relref "x" >}}` /
  `{{< ref "x" >}}` and site-absolute links name a content page — find the matching file
  under `references/`, or browse https://longhorn.io/ + the path.
- The live URL of a page ≈ https://longhorn.io/ + its `references/`-relative path without `.md`
  (`_index.md` → the directory URL); use that when citing sources.
- Content was extracted from the site at generation time and may have drifted since.

## Contents

### [The Longhorn Knowledge Base](references/kb/_index.md) — 62 pages

- [Analysis: Potential Data/Filesystem Corruption](references/kb/analysis-filesystem-corrupted-issues-due-to-error-on-cow-while-rebuilding-replicas.md)
- [Backup store lock conflict error message](references/kb/backup-store-lock-conflict-error-message.md)
- [Troubleshooting: `volume readonly or I/O error`](references/kb/troubleshooting-volume-readonly-or-io-error.md)
- ...

Anything not covered here: https://longhorn.io/
```

Small sites list every page in `SKILL.md` (like above); sites with more than 100 pages get top-level section links instead, and the per-section `_index.md` files carry the page listings — `SKILL.md` stays under the spec's 500-line guidance by construction.

# Build this project

```bash
CGO_ENABLED=0 go build
```

# Run tests

```bash
CGO_ENABLED=0 go test ./...
```

Golden files under `pkg/skill/testdata/golden/` are regenerated with:

```bash
go test ./pkg/skill/ -update
```

## Docker

Docker images are available at
[DockerHub](https://hub.docker.com/r/yardenshoham/hugo-to-skill)
(docker.io/yardenshoham/hugo-to-skill).

Available docker tags

| Tag      | Description                                 |
| -------- | ------------------------------------------- |
| `latest` | latest available release of hugo-to-skill.  |
| `va.b.c` | hugo-to-skill version `a.b.c` .             |
| `a.b.c`  | hugo-to-skill version `a.b.c` .             |

### Docker run

```shell script
docker run \
    -v $PWD:/workdir \
    yardenshoham/hugo-to-skill:latest generate https://github.com/longhorn/website --content-path kb --output /workdir/skills/longhorn-kb
```

### Docker build

You can build an own docker image by running

```shell
CGO_ENABLED=0 go build && docker build -t hugo-to-skill .
```
