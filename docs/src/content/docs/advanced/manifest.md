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
| `.json` | JSON |
| `.yaml`, `.yml` (or anything else) | YAML |

## Schema

The manifest contains the following fields:

| Field | Description |
|-------|-------------|
| `SchemaVersion` | URL pointing to the [Manifest Schema](#manifest-schema) for the manifest format |
| `Timestamp` | UTC timestamp of the generation run (RFC 3339) |
| `TemplateURL` | The `--template-url` value used for this run |
| `BoilerplateVersion` | Version of boilerplate that produced the output |
| `OutputDir` | The `--output-folder` value used for this run |
| `Files` | Array of generated files |
| `Files[].Path` | Path of the generated file, relative to the output directory |
| `Files[].Checksum` | SHA256 hex digest of the file contents |

### YAML example

```yaml
SchemaVersion: "https://boilerplate.gruntwork.io/schemas/manifest/v1/schema.json"
Timestamp: "2026-02-24T12:00:00Z"
TemplateURL: ./templates/service
BoilerplateVersion: v0.6.0
OutputDir: ./output
Files:
  - Path: main.go
    Checksum: a1b2c3d4e5f6...
  - Path: README.md
    Checksum: f6e5d4c3b2a1...
```

### JSON example

```json
{
  "SchemaVersion": "https://boilerplate.gruntwork.io/schemas/manifest/v1/schema.json",
  "Timestamp": "2026-02-24T12:00:00Z",
  "TemplateURL": "./templates/service",
  "BoilerplateVersion": "v0.6.0",
  "OutputDir": "./output",
  "Files": [
    {
      "Path": "main.go",
      "Checksum": "a1b2c3d4e5f6..."
    },
    {
      "Path": "README.md",
      "Checksum": "f6e5d4c3b2a1..."
    }
  ]
}
```

## Manifest Schema

Boilerplate publishes a formal [JSON Schema](https://json-schema.org/) for the manifest format. The schema is available at:

```
https://boilerplate.gruntwork.io/schemas/manifest/v1/schema.json
```

The `SchemaVersion` field in every generated manifest contains this URL, making it easy to identify which schema version was used and to fetch the schema for validation.

## Behavior

- **Overwrite** — Each run overwrites the previous manifest. There is no version history; the manifest always reflects the most recent generation.
- **Corrupt file detection** — If an existing manifest file is present but contains invalid content, boilerplate exits with an error rather than silently overwriting it. This protects against accidentally clobbering a file that was not actually a manifest.
- **Checksums** — Checksums are computed after all files have been generated, using streaming SHA256 over each file. Binary files (images, compiled assets) are checksummed the same way as text files.
