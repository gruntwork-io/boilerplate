// Demo of boilerplateInputsMap from Node.js. Usage:
//   make wasm
//   node examples/wasm/node/inputs_demo.mjs
//
// Mirrors demo.mjs (renderTemplate) but exercises the static-analysis
// subcommand instead. The bundle here is the equivalent of the test fixture
// at test-fixtures/inputs-test/transitive — a root template with one nested
// dependency that receives a parent variable via an explicit value
// expression.

import { readFile } from "node:fs/promises";
import { brotliDecompress } from "node:zlib";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const decompress = promisify(brotliDecompress);
const __dirname = dirname(fileURLToPath(import.meta.url));

await import(join(__dirname, "wasm_exec.js"));

const compressed = await readFile(join(__dirname, "boilerplate.wasm.br"));
const wasmBytes = await decompress(compressed);

const go = new Go();
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
go.run(instance);

const bundle = {
  rootPath: ".",
  files: {
    "boilerplate.yml": [
      "variables:",
      "  - name: Region",
      "  - name: ProjectName",
      "dependencies:",
      "  - name: vpc",
      "    template-url: ./modules/vpc",
      "    output-folder: ./modules/vpc",
      "    variables:",
      '      - name: AwsRegion',
      '        default: "{{ .Region }}"',
    ].join("\n"),
    "README.md": "# {{ .ProjectName }}\nDeployed in {{ .Region }}.\n",
    "modules/vpc/boilerplate.yml": "variables:\n  - name: AwsRegion\n",
    "modules/vpc/main.tf": 'provider "aws" { region = "{{ .AwsRegion }}" }\n',
  },
};

const vars = JSON.stringify({ Region: "us-east-1", ProjectName: "demo" });

const out = globalThis.boilerplateInputsMap(JSON.stringify(bundle), vars);

if (out instanceof Error) {
  console.error("Error:", out.message);
  process.exit(1);
}

const parsed = JSON.parse(out);
console.log(JSON.stringify(parsed, null, 2));
