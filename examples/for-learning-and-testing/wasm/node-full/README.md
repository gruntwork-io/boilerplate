# Node.js Demo (full build)

Command-line script that runs the full boilerplate pipeline (`boilerplateProcessTemplate`) against a template folder on disk, using the larger `boilerplate-full.wasm` build.

This is the WASM equivalent of running `boilerplate --template-url ./template --output-folder /tmp/out --var Name=world ...` from the CLI. Use this build when you need directory walks, `boilerplate.yml`, dependencies, partials, or manifest output. If you only need to render a single template string, use the lite demo in `../node/` instead — it's roughly 6x smaller.

## Run

First, build the WASM artifacts from the repository root:

```sh
make wasm
```

Then run the demo:

```sh
node examples/wasm/node-full/demo.mjs
```

## What it does

1. Wires `globalThis.fs`, `globalThis.path`, and `globalThis.crypto` to real Node implementations *before* importing `wasm_exec.js`. The bundled `wasm_exec.js` only sets stub fs that return `ENOSYS`, which is enough for `renderTemplate` but breaks anything that reads or writes files.
2. Loads the brotli-compressed `boilerplate-full.wasm.br` and instantiates the Go runtime.
3. Calls `boilerplateProcessTemplate` against `./template/`, writing into a freshly-created temp directory.
4. Prints the path of each generated file and its contents.

The temp output folder is removed on exit.

## Template layout

`template/boilerplate.yml` declares the input variables; `template/greeting.txt` is the file that gets rendered. The demo passes `Name="world"` and `Items=["alpha", "bravo", "charlie"]` as the variable values — same data shape as the lite demo, but routed through the full pipeline.
