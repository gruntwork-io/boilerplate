---
title: Manifest
sidebar:
  order: 5
description: Track generated files with a manifest that includes SHA256 checksums.
---

## Overview

Boilerplate can produce a **manifest file** that records every file it generated during a run, along with SHA256 checksums for each file. This is useful for:

- **Auditing** — knowing exactly which files were created by a template
- **Drift detection** — comparing checksums to see if generated files were modified after the fact
- **CI/CD pipelines** — programmatically consuming the list of generated files in downstream steps

## Enabling the Manifest

Use the `--manifest` flag to write a manifest to the output directory:

```bash
boilerplate \
  --template-url ./templates/service \
  --output-folder ./output \
  --non-interactive \
  --manifest
```

This creates `boilerplate-manifest.yaml` inside the output folder.

To write the manifest to a custom path, use `--manifest-file`:

```bash
boilerplate \
  --template-url ./templates/service \
  --output-folder ./output \
  --non-interactive \
  --manifest-file ./reports/manifest.yaml
```

`--manifest-file` implies `--manifest`, so you don't need to pass both.

## Format Auto-Detection

The manifest format is determined by the file extension:

| Extension | Format |
|-----------|--------|
| `.yaml`, `.yml` | YAML |
| `.json` (or anything else) | JSON |

## Schema

The manifest contains the following fields:

| Field | Description |
|-------|-------------|
| `schema_version` | Manifest schema version (currently `1.0`) |
| `timestamp` | UTC timestamp of the generation run (RFC 3339) |
| `template_url` | The `--template-url` value used for this run |
| `boilerplate_version` | Version of boilerplate that produced the output |
| `output_dir` | The `--output-folder` value used for this run |
| `files` | Array of generated files |
| `files[].path` | Path of the generated file, relative to the output directory |
| `files[].checksum` | SHA256 hex digest of the file contents |

### JSON example

```json
{
  "schema_version": "1.0",
  "timestamp": "2026-02-24T12:00:00Z",
  "template_url": "./templates/service",
  "boilerplate_version": "v0.6.0",
  "output_dir": "./output",
  "files": [
    {
      "path": "main.go",
      "checksum": "a1b2c3d4e5f6..."
    },
    {
      "path": "README.md",
      "checksum": "f6e5d4c3b2a1..."
    }
  ]
}
```

### YAML example

```yaml
schema_version: "1.0"
timestamp: "2026-02-24T12:00:00Z"
template_url: ./templates/service
boilerplate_version: v0.6.0
output_dir: ./output
files:
  - path: main.go
    checksum: a1b2c3d4e5f6...
  - path: README.md
    checksum: f6e5d4c3b2a1...
```

## Behavior

- **Overwrite** — Each run overwrites the previous manifest. There is no version history; the manifest always reflects the most recent generation.
- **Corrupt file detection** — If an existing manifest file is present but contains invalid content, boilerplate exits with an error rather than silently overwriting it. This protects against accidentally clobbering a file that was not actually a manifest.
- **Checksums** — Checksums are computed after all files have been generated, using streaming SHA256 over each file. Binary files (images, compiled assets) are checksummed the same way as text files.
