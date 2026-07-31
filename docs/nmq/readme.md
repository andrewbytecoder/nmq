# NCP MkDocs

This directory contains the MkDocs site for `ncp`.

## Local preview

```bash
cd docs/ncp
mkdocs serve -f mkdocs.yml
```

## Build output

```bash
cd docs/ncp
mkdocs build -f mkdocs.yml
```

## Content sources

The pages in this site are derived from the repository itself:

- `cmd/`, `plugins/`, `interfaces/`, `internal/`, `pkg/`
- `manifest/`
- `README.md`
- `docs/概要设计/`
