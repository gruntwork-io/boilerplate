---
title: CI/CD Integration
sidebar:
  order: 3
description: Using Boilerplate in continuous integration and deployment pipelines.
---

## Overview

Boilerplate is designed to work well in automated pipelines. The `--non-interactive` flag ensures all variables
are provided explicitly, making builds reproducible and deterministic.

## Non-Interactive Mode

In CI/CD, always use `--non-interactive` to prevent Boilerplate from waiting for user input:

```bash
boilerplate \
  --template-url ./templates/deploy \
  --output-folder ./generated \
  --non-interactive \
  --var-file vars/production.yml \
  --var Version="${CI_COMMIT_TAG}"
```

In this mode:
- All variables must have a value from `--var`, `--var-file`, environment variables, or defaults
- Missing variables cause an immediate error (no prompts)
- Hooks are auto-approved
- Dependencies are auto-included

## Variable Files per Environment

A common pattern is to maintain variable files for each environment:

```
vars/
├── common.yml
├── dev.yml
├── staging.yml
└── production.yml
```

```bash
boilerplate \
  --template-url ./templates/infra \
  --output-folder ./generated \
  --non-interactive \
  --var-file vars/common.yml \
  --var-file "vars/${ENVIRONMENT}.yml"
```

Later `--var-file` values override earlier ones, so environment-specific values override common defaults.

## Environment Variables

Boilerplate checks for environment variables in the format `BOILERPLATE_VAR_<NAME>`:

```bash
export BOILERPLATE_VAR_DatabaseHost="db.example.com"
export BOILERPLATE_VAR_ApiKey="secret123"

boilerplate \
  --template-url ./templates/config \
  --output-folder ./generated \
  --non-interactive
```

## GitHub Actions Example

```yaml
jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Boilerplate
        run: |
          curl -Lo boilerplate https://github.com/gruntwork-io/boilerplate/releases/latest/download/boilerplate_linux_amd64
          chmod +x boilerplate
          sudo mv boilerplate /usr/local/bin/

      - name: Generate Configuration
        run: |
          boilerplate \
            --template-url ./templates/deploy \
            --output-folder ./generated \
            --non-interactive \
            --var-file vars/production.yml \
            --var Version="${{ github.ref_name }}"
```

## Disabling Hooks and Shell

For security in CI/CD, you may want to disable hooks or shell execution:

```bash
# Skip all hooks
boilerplate --no-hooks ...

# Disable shell helper (returns "replace-me" instead of executing)
boilerplate --no-shell ...
```
