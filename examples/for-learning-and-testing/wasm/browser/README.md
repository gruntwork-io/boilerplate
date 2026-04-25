# Browser Demo

Interactive demo page that renders Go templates in the browser using boilerplate compiled to WebAssembly.

## Run locally

First, build the WASM artifacts from the repository root:

```sh
make wasm
```

Then serve the example directory with any static file server:

```sh
python3 -m http.server 8080 -d examples/wasm/browser
```

Open http://localhost:8080 in your browser.

## How it works

The demo page (`index.html`) feature-detects the browser's `DecompressionStream` API for brotli support:

- **Supported** (Chrome/Edge 120+): Fetches the smaller `boilerplate.wasm.br` and decompresses it natively via `DecompressionStream("brotli")`.
- **Not supported** (Firefox, Safari): Falls back to fetching the uncompressed `boilerplate.wasm` directly.

Once loaded, the page exposes a `boilerplateRenderTemplate(template, varsJSON)` function that the render button calls to process the template with the provided variables.
