import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { zip } from "./zip.mjs";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = resolve(root, "examples/hello-pier.zip");
const entries = await Promise.all(
  ["upy.yaml", "dist/index.html"].map(async (name) => ({
    name,
    data: await readFile(resolve(root, "examples/static", name)),
  })),
);
await writeFile(output, zip(entries));
console.log(output);
