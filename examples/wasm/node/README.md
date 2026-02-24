# Node.js Demo

Command-line script that renders a Go template using boilerplate compiled to WebAssembly.

## Run

First, build the WASM artifacts from the repository root:

```sh
make wasm
```

Then run the demo:

```sh
node examples/wasm/node/demo.mjs
```

## What it does

The script loads the boilerplate WASM binary, instantiates the Go runtime, and calls `boilerplateRenderTemplate` with a sample template and JSON variables — the same ones used in the browser demo. The rendered output is printed to stdout.
