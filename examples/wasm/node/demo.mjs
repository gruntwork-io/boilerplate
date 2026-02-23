import { readFile } from "node:fs/promises";
import { brotliDecompress } from "node:zlib";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const decompress = promisify(brotliDecompress);
const __dirname = dirname(fileURLToPath(import.meta.url));

// wasm_exec.js sets globalThis.Go as a side effect
await import(join(__dirname, "wasm_exec.js"));

const compressed = await readFile(join(__dirname, "boilerplate.wasm.br"));
const wasmBytes = await decompress(compressed);

const go = new Go();
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);

// Don't await go.run() — Go's main ends with select{} so it blocks forever
go.run(instance);

const template = `Hello, {{ .Name | upper }}!

Items:
{{ range .Items }}- {{ . }}
{{ end }}
Written by {{ .Author }}.`;

const vars = JSON.stringify({
  Name: "world",
  Items: ["alpha", "bravo", "charlie"],
  Author: "boilerplate",
});

const result = globalThis.boilerplateRenderTemplate(template, vars);

if (result instanceof Error) {
  console.error("Error:", result.message);
  process.exit(1);
}

console.log(result);
