---
title: Terminology
sidebar:
  order: 2
description: Definitions of key terms used throughout the Boilerplate documentation.
---

This page defines the terms used throughout the Boilerplate documentation. Using these terms consistently helps avoid confusion, especially when Boilerplate is used alongside tools like Terragrunt that have their own vocabulary.

## Template

A directory containing a `boilerplate.yml` configuration file and one or more files with Go template syntax. Templates are the core unit in Boilerplate — everything starts with a template.

```
my-template/
├── boilerplate.yml
├── README.md
└── main.go
```

A template is referenced by its path using `--template-url`:

```bash
boilerplate --template-url ./my-template --output-folder ./output
```

## Variable

A named value declared in `boilerplate.yml` that gets substituted into template files during rendering. Variables have a name, type, and optional default. Boilerplate supports seven variable types: `string`, `int`, `float`, `bool`, `list`, `map`, and `enum`.

```yaml
variables:
  - name: ProjectName
    type: string
    description: The name of the project
```

Variables are referenced in template files with `{{ .ProjectName }}`.

## Dependency

A reference to another template that runs as part of the current template. Dependencies are declared in `boilerplate.yml` and let you compose larger templates from smaller, reusable ones.

```yaml
dependencies:
  - name: backend
    template-url: ../go-service
    output-folder: ./backend
```

The template being referenced by a dependency is still called a **template** — not a "module." When you need to distinguish between the two, use **parent template** (the one declaring the dependency) and **child template** (the one being pulled in).

### Why not "module"?

The term "module" is intentionally avoided in Boilerplate's vocabulary because it conflicts with OpenTofu/Terraform modules, which are a common target for Boilerplate-generated code. When someone says "module" in the context of infrastructure-as-code, it almost always means an OpenTofu/Terraform module. Calling Boilerplate's dependencies "modules" would create ambiguity, especially when using Boilerplate with [Terragrunt Scaffold](/intro/terragrunt/), which generates configurations for OpenTofu/Terraform modules.

Use **template** or **dependency** instead.

## Hook

A shell command that runs before or after template rendering. Hooks are useful for formatting generated code, installing dependencies, or running validation.

```yaml
hooks:
  before:
    - command: echo
      args: ["Generating files..."]
  after:
    - command: gofmt
      args: ["-w", "."]
```

## Partial

A reusable template fragment that can be included in multiple template files using the Go template `{{ template }}` action. Partials are declared in `boilerplate.yml` and help avoid duplicating common template logic.

## Output Folder

The directory where Boilerplate writes rendered files, specified with `--output-folder`. The output folder mirrors the structure of the template directory, with all Go template expressions resolved.

## Template URL

The path or URL to a template directory, specified with `--template-url`. This can be a local file path, a Git repository URL, an S3/GCS path, or any URL supported by [go-getter](https://github.com/hashicorp/go-getter). See [Remote Templates](/advanced/remote-templates/) for details.

## Quick Reference

| Term | Definition |
|------|-----------|
| **Template** | A directory with a `boilerplate.yml` and template files |
| **Variable** | A named, typed value substituted into templates |
| **Dependency** | A reference to a child template that runs as part of a parent |
| **Parent template** | The template that declares dependencies |
| **Child template** | The template referenced by a dependency |
| **Hook** | A shell command that runs before or after rendering |
| **Partial** | A reusable template fragment included via `{{ template }}` |
| **Output folder** | The directory where rendered files are written |
| **Template URL** | The path or URL to a template directory |
