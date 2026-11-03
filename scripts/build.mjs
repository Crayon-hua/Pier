import { spawnSync } from "node:child_process";
import { cpSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
function run(command, args, cwd) {
  const result = spawnSync(command, args, {
    cwd,
    stdio: "inherit",
    windowsHide: true,
    shell: process.platform === "win32" && command === "npm",
  });
  if (result.status !== 0) process.exit(result.status || 1);
}
run("npm", ["run", "build"], resolve(root, "web"));
mkdirSync(resolve(root, "server/internal/webembed/dist"), { recursive: true });
cpSync(
  resolve(root, "web/dist"),
  resolve(root, "server/internal/webembed/dist"),
  { recursive: true },
);
const suffix = process.platform === "win32" ? ".exe" : "";
run(
  "go",
  ["build", "-trimpath", "-o", `bin/pier${suffix}`, "./cmd/server"],
  resolve(root, "server"),
);
run(
  "go",
  ["build", "-trimpath", "-o", `bin/pier-crypt${suffix}`, "./cmd/pier-crypt"],
  resolve(root, "server"),
);
console.log(`Built: ${resolve(root, "server/bin")}`);
