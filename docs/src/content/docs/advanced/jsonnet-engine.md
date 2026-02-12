---
title: Jsonnet Engine
sidebar:
  order: 2
description: Use Jsonnet as an alternative template engine for JSON generation.
---

## Overview

Boilerplate supports [Jsonnet](https://jsonnet.org/) as an alternative template engine for generating JSON files.
Jsonnet provides functions, imports, and a more structured approach to JSON generation compared to Go templates.

## Enabling Jsonnet

Add an `engines` section to your `boilerplate.yml`:

```yaml
engines:
  - path: "**/*.jsonnet"
    template_engine: jsonnet
```

## How It Works

1. Files matching the engine's `path` glob are processed with the Jsonnet engine instead of Go templates
2. Boilerplate variables are passed as Jsonnet Top Level Arguments (TLAs)
3. The `.jsonnet` extension is automatically stripped from output filenames

So `config.json.jsonnet` becomes `config.json` in the output.

## Example

### boilerplate.yml

```yaml
variables:
  - name: ServiceName
    type: string
  - name: Port
    type: int
    default: 8080
  - name: Replicas
    type: int
    default: 3

engines:
  - path: "**/*.jsonnet"
    template_engine: jsonnet
```

### k8s-deployment.json.jsonnet

```jsonnet
function(ServiceName, Port, Replicas)
{
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: {
    name: ServiceName,
  },
  spec: {
    replicas: std.parseInt(Replicas),
    template: {
      spec: {
        containers: [{
          name: ServiceName,
          port: std.parseInt(Port),
        }],
      },
    },
  },
}
```

## Variable Access

In Jsonnet templates, variables are passed as string TLAs. Use `std.parseInt()` or `std.parseJson()` to convert
types as needed.

## Mixing Engines

You can use both Go templates and Jsonnet in the same template directory. Files matching the Jsonnet engine glob
use Jsonnet; all other files use Go templates.

```yaml
engines:
  - path: "**/*.jsonnet"
    template_engine: jsonnet

# Regular .go, .md, .yaml files still use Go templates
```
