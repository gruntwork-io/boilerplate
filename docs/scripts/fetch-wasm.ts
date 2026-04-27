// Downloads the WASM artifacts attached to the latest GitHub Release into
// docs/public/wasm/ so the embedding-page demo can serve them same-origin at
// build time. Skips files that already exist, so repeat local builds are fast.

import { resolve } from "node:path";

const outDir = resolve(import.meta.dir, "..", "public", "wasm");
const base = "https://github.com/gruntwork-io/boilerplate/releases/latest/download";
const assets = ["boilerplate.wasm", "wasm_exec.js"];

for (const name of assets) {
	const dest = resolve(outDir, name);

	if (await Bun.file(dest).exists()) {
		console.log(`fetch-wasm: ${name} already present, skipping`);
		continue;
	}

	const url = `${base}/${name}`;
	console.log(`fetch-wasm: downloading ${url}`);

	const res = await fetch(url, { redirect: "follow" });
	if (!res.ok) {
		throw new Error(`failed to fetch ${url}: ${res.status} ${res.statusText}`);
	}

	const written = await Bun.write(dest, res);
	console.log(`fetch-wasm: wrote ${dest} (${written} bytes)`);
}
