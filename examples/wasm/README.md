# Boilerplate WASM Demo

This example runs boilerplate in the browser via WebAssembly. You can enter a Go template and JSON variables, then render the result entirely client-side.

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
| `boilerplate.wasm` | Uncompressed WASM binary (~13 MB) |
| `boilerplate.wasm.br` | Brotli-compressed WASM binary (~2 MB) |
| `wasm_exec.js` | Go's WASM runtime support (copied from `GOROOT`) |

## Run locally

Serve the example directory with any static file server:

```sh
python3 -m http.server 8080 -d examples/wasm
```

Then open http://localhost:8080 in your browser.

## How it works

The demo page (`index.html`) feature-detects the browser's `DecompressionStream` API for brotli support:

- **Supported** (Chrome/Edge 120+): Fetches the smaller `boilerplate.wasm.br` and decompresses it natively via `DecompressionStream("brotli")`.
- **Not supported** (Firefox, Safari): Falls back to fetching the uncompressed `boilerplate.wasm` directly.

Once loaded, the page exposes a `boilerplateRenderTemplate(template, varsJSON)` function that the render button calls to process the template with the provided variables.
