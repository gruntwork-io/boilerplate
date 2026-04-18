# Boilerplate WASM Examples

This directory contains examples of running boilerplate via WebAssembly in both browser and Node.js environments.

## Prerequisites

- Go (see `.mise.toml` at the repo root for the expected version)
- `brotli` CLI for compression: `brew install brotli` (macOS) or `apt-get install brotli` (Linux)

## Build

From the repository root:

```sh
make wasm
```

This compiles the WASM binary, compresses it with brotli, and copies the Go WASM support JS into `examples/wasm/`. The resulting files are:

| File | Description |
|---|---|
| `boilerplate.wasm` | Uncompressed WASM binary |
| `boilerplate.wasm.br` | Brotli-compressed WASM binary |
| `wasm_exec.js` | Go's WASM runtime support (copied from `GOROOT`) |

## JS API

Loading the WASM binary registers two functions on `globalThis`:

### `boilerplateRenderTemplate(templateStr, varsJSON) => string`

Synchronously renders a single Go template string against a map of variables. Returns the rendered string, or a `js.Error` if arguments are malformed or rendering fails. This wraps `render.RenderTemplateFromString` — no directory walk, no `boilerplate.yml`, no dependencies.

### `boilerplateProcessTemplate(optionsJSON) => Promise<Result>`

Runs the full `templates.ProcessTemplateWithContext` pipeline against a template folder on the host filesystem. Use this when you need feature parity with the boilerplate CLI — directory walks, `skip_files`, dependencies, partials, path-name templating, manifest generation, etc. — without the overhead of spawning a subprocess.

Because Go's filesystem syscalls are asynchronous under `GOOS=js`, the function returns a Promise that you must `await`.

`optionsJSON` is a JSON-encoded object:

| Field | Type | Default | Description |
|---|---|---|---|
| `templateFolder` | string | **required** | Local path to the template directory. |
| `outputFolder` | string | **required** | Local path that generated files will be written to. |
| `vars` | object | `{}` | Variable name → value. Merged on top of `boilerplate.yml` defaults. |
| `varFiles` | string[] | `[]` | Paths to YAML variable files. Values from files override inline `vars` (matches CLI precedence). |
| `nonInteractive` | bool | `true` | Never prompt. Prompts would deadlock in WASM anyway. |
| `noShell` | bool | `true` | Block shell hooks. |
| `disableDependencyPrompt` | bool | `true` | Skip confirmation prompt for dependency runs. |
| `onMissingKey` | `"invalid"` \| `"zero"` \| `"error"` | `"error"` | Behavior when a template references an undefined variable. |
| `onMissingConfig` | `"exit"` \| `"ignore"` | `"ignore"` | Behavior when the template folder has no `boilerplate.yml`. |
| `manifest` | bool | `false` | Emit a manifest file describing the run. |

Anything not listed above uses `BoilerplateOptions` defaults.

The resolved `Result` object is:

```ts
{
  error: string,            // empty on success; failure message otherwise
  generatedFiles: string[], // paths to files written by this run
  sourceChecksum: string,   // populated only when manifest=true
}
```

Argument-shape failures (wrong arity, invalid JSON, invalid enum values) reject the Promise with a JS `Error`. Render failures resolve the Promise with a populated `error` field, so callers can branch on a field instead of wrapping in `try`/`catch`.

### Host setup for Node

The shipped `wasm_exec.js` is the generic Go runtime. Under Node, you must populate a handful of globals before loading it so the Go runtime can find real implementations of `fs`, `path`, `crypto`, etc. See `node/demo.mjs` for the pattern.

## Examples

- **[Browser](browser/)** — Interactive demo page that renders Go templates client-side.
- **[Node.js](node/)** — Command-line script that renders a Go template using Node.js.
