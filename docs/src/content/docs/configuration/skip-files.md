---
title: Skip Files
sidebar:
  order: 5
description: Exclude files from the rendered output based on conditions.
---

## Overview

The `skip_files` section in `boilerplate.yml` lets you conditionally exclude files from the output. This is useful for
optional features like Docker support, test files, or platform-specific configuration.

## Syntax

### Skip by path (exclude files)

```yaml
skip_files:
  - path: "Dockerfile"
    if: "{{ not .IncludeDocker }}"

  - path: "*.test.go"
    if: "{{ not .IncludeTests }}"
```

When `if` evaluates to `true`, the matching files are **skipped** (excluded from output).

### Skip by not_path (include only specific files)

```yaml
skip_files:
  - not_path: "*.go"
    if: true
```

When using `not_path`, all files that do **not** match the pattern are skipped.

## Glob Patterns

Both `path` and `not_path` support glob patterns:

| Pattern | Matches |
|---------|---------|
| `Dockerfile` | Exact filename |
| `*.test.go` | All files ending in `.test.go` |
| `configs/*` | All files directly inside `configs/` |
| `**/*.yaml` | All `.yaml` files in any subdirectory |

## Conditional Expressions

The `if` field uses Go template syntax and can access all template variables:

```yaml
skip_files:
  # Skip if not production
  - path: "monitoring/*"
    if: '{{ ne .Environment "production" }}'

  # Always skip
  - path: ".gitkeep"
    if: "true"

  # Skip based on multiple conditions
  - path: "docker-compose.yml"
    if: "{{ or (not .IncludeDocker) .UseKubernetes }}"
```

## Unconditional Skip

To always skip certain files, set `if` to the string `"true"`:

```yaml
skip_files:
  - path: "boilerplate.yml"
    if: "true"
```
