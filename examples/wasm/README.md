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
| `boilerplate.wasm` | Uncompressed WASM binary (~13 MB) |
| `boilerplate.wasm.br` | Brotli-compressed WASM binary (~2 MB) |
| `wasm_exec.js` | Go's WASM runtime support (copied from `GOROOT`) |

## Examples

- **[Browser](browser/)** — Interactive demo page that renders Go templates client-side.
- **[Node.js](node/)** — Command-line script that renders a Go template using Node.js.
