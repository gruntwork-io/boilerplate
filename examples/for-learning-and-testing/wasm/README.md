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

This compiles both WASM binaries, compresses them with brotli, and copies the Go WASM support JS into `examples/wasm/`. The resulting files are:

| File | Description |
|---|---|
| `boilerplate.wasm` | Lite build — only `boilerplateRenderTemplate` |
| `boilerplate.wasm.br` | Brotli-compressed lite build |
| `boilerplate-full.wasm` | Full build — adds `boilerplateProcessTemplate` |
| `boilerplate-full.wasm.br` | Brotli-compressed full build |
| `wasm_exec.js` | Go's WASM runtime support (copied from `GOROOT`) |

The lite build excludes the `templates` package and its dependencies, so it is roughly 6x smaller than the full build. Choose lite if you only need to render template strings; choose full if you need the directory-walking pipeline (`boilerplate.yml`, dependencies, partials, manifest, etc.).

You can also build them individually:

```sh
make build-wasm-lite   # boilerplate.wasm only
make build-wasm-full   # boilerplate-full.wasm only
```

## JS API

### `boilerplateRenderTemplate(templateStr, varsJSON) => string`

Available in **both lite and full** builds.

Synchronously renders a single Go template string against a map of variables. Returns the rendered string, or a `js.Error` if arguments are malformed or rendering fails. This wraps `render.RenderTemplateFromString` — no directory walk, no `boilerplate.yml`, no dependencies.

### `boilerplateProcessTemplate(optionsJSON) => Promise<Result>`

Available in the **full** build only.

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

#### Defaults that diverge from the CLI

A handful of defaults are deliberately stricter than the CLI (`cli/parse_options.go`). They are not bugs:

| Field | WASM default | CLI default | Why |
|---|---|---|---|
| `nonInteractive` | `true` | `false` | Prompts call TTY code that would deadlock the Go runtime under `GOOS=js` (JS event loop is blocked on our `FuncOf` callback). |
| `noShell` | `true` | `false` | No host shell exists under `GOOS=js`; hooks would fail noisily. |
| `disableDependencyPrompt` | `true` | `false` | Same deadlock risk as `nonInteractive`. |
| `onMissingConfig` | `"ignore"` | `"exit"` | WASM callers frequently invoke against plain template folders with no `boilerplate.yml`; failing hard breaks the common case. |

The resolved `Result` object is:

```ts
{
  error: string,            // empty on success; failure message otherwise
  generatedFiles: string[], // paths to files written by this run
  sourceChecksum: string,   // populated only when manifest=true
  warnings: string[],       // non-fatal notices (e.g. custom validations skipped in WASM)
}
```

Argument-shape failures (wrong arity, invalid JSON, invalid enum values) reject the Promise with a JS `Error`. Render failures resolve the Promise with a populated `error` field, so callers can branch on a field instead of wrapping in `try`/`catch`.

#### WASM-specific caveats

- **Custom `validations` on variables are not enforced.** The ozzo-validation library pulls in transitive crypto code that is excluded from the WASM binary, so the `runValidation` helper is a no-op under `GOOS=js`. If a template declares `validations`, the run emits a `warnings` entry and proceeds.
- **Jsonnet templates are not supported.** `google/go-jsonnet` does not compile under `GOOS=js`; any `.jsonnet` in the template folder produces an error rather than being silently skipped.

### Host setup for Node

The shipped `wasm_exec.js` is the generic Go runtime. Under Node, you must populate a handful of globals before loading it so the Go runtime can find real implementations of `fs`, `path`, `crypto`, etc. See `node/demo.mjs` for the pattern.

## Examples

- **[Browser](browser/)** — Interactive demo page that renders Go templates client-side (lite build).
- **[Node.js](node/)** — Command-line script that renders a Go template using Node.js (lite build).
- **[Node.js, full build](node-full/)** — Runs the full `boilerplateProcessTemplate` pipeline against a template folder on disk. Includes the host-side `globalThis.fs/path/crypto` wiring required for fs syscalls.
