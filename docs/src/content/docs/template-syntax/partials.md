---
title: Partials
sidebar:
  order: 3
description: Reusable template fragments with Go template partials.
---

## Overview

Partials are reusable template fragments defined in external files. They let you share common sections
(headers, footers, license blocks) across multiple templates without duplicating code.

## Defining a Partial

Create a file with a Go template `define` block:

```text
<!-- partials/license.html -->
{{ define "license" }}
Copyright {{ .Year }} {{ .Company }}. All rights reserved.
Licensed under the {{ .License }} License.
{{ end }}
```

## Registering Partials

Register partials in your `boilerplate.yml` using glob patterns:

```yaml
partials:
  - ../../partials/*.html
  - ../shared/*.txt
```

## Using Partials

Include a partial in any template file:

```text
{{ template "license" . }}
```

The `.` passes the current template variables to the partial.

## How It Works

1. Boilerplate loads all files matching the `partials` globs
2. It parses each file for `{{ define "name" }}` blocks
3. These named templates become available to all template files via `{{ template "name" . }}`
4. If two partials define the same name, the last one loaded wins

## Checking if a Partial Exists

Use the `templateIsDefined` helper to conditionally include a partial:

```text
{{ if templateIsDefined "custom-header" }}
{{ template "custom-header" . }}
{{ else }}
# Default Header
{{ end }}
```

## Dynamic Partial Paths

Partial glob paths in `boilerplate.yml` support Go template syntax with the convenience variables `templateFolder` and `outputFolder`:

```yaml
partials:
  - "{{ templateFolder }}/../shared-partials/*.html"
  - "{{ outputFolder }}/generated-partials/*.tmpl"
```

This is useful when your partials live outside the template directory and you need to reference them relative to the template or output location.
