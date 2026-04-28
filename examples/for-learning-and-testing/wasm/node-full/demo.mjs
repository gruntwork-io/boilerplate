// Demo of the full WASM build (boilerplate-full.wasm), which exposes
// boilerplateProcessTemplate in addition to boilerplateRenderTemplate.
//
// Unlike the lite demo, processTemplate walks a template folder on disk and
// writes the rendered output to another folder, so we have to wire the Go
// runtime's fs/path/crypto globals to real Node implementations *before*
// loading wasm_exec.js. The bundled wasm_exec.js stubs out fs with ENOSYS
// returns; that is fine for renderTemplate (no fs touched) but breaks
// anything that actually reads or writes files.

import { readFile, mkdtemp, rm } from "node:fs/promises";
import * as nodeFs from "node:fs";
import * as nodePath from "node:path";
import { brotliDecompress } from "node:zlib";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { tmpdir } from "node:os";
import { webcrypto } from "node:crypto";

const decompress = promisify(brotliDecompress);
const __dirname = dirname(fileURLToPath(import.meta.url));

// Wire host globals so the Go runtime can find real implementations.
// wasm_exec.js only installs its own stubs when these are unset, so order
// matters: set them first, then import wasm_exec.js.
globalThis.fs = nodeFs;
globalThis.path = nodePath;
if (!globalThis.crypto) {
  globalThis.crypto = webcrypto;
}

await import(join(__dirname, "wasm_exec.js"));

const compressed = await readFile(join(__dirname, "boilerplate-full.wasm.br"));
const wasmBytes = await decompress(compressed);

const go = new Go();
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);

// Don't await go.run() — Go's main ends with select{} so it blocks forever.
go.run(instance);

// Yield until Go's main has run and registered the global. go.run is
// fire-and-forget, so the function isn't installed synchronously.
for (let i = 0; i < 100 && typeof globalThis.boilerplateProcessTemplate !== "function"; i++) {
  await new Promise((r) => setImmediate(r));
}
if (typeof globalThis.boilerplateProcessTemplate !== "function") {
  throw new Error("boilerplateProcessTemplate did not register within timeout");
}

const templateFolder = join(__dirname, "template");
const outputFolder = await mkdtemp(join(tmpdir(), "boilerplate-demo-"));

const request = JSON.stringify({
  templateFolder,
  outputFolder,
  vars: {
    Name: "world",
    Items: ["alpha", "bravo", "charlie"],
  },
});

try {
  const result = await globalThis.boilerplateProcessTemplate(request);

  if (result.error) {
    console.error("processTemplate failed:", result.error);
    process.exit(1);
  }

  console.log(`Output folder: ${outputFolder}`);
  console.log(`Generated files: ${result.generatedFiles.length}`);

  // generatedFiles entries are relative to outputFolder.
  for (const relPath of result.generatedFiles) {
    const absPath = join(outputFolder, relPath);
    const contents = await readFile(absPath, "utf8");
    console.log(`\n--- ${relPath} ---\n${contents}`);
  }

  if (result.warnings.length > 0) {
    console.log("\nWarnings:");
    for (const w of result.warnings) {
      console.log(`  - ${w}`);
    }
  }
} finally {
  await rm(outputFolder, { recursive: true, force: true });
}
