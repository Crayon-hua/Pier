// Real HTTP + worker process smoke test. Uses a disposable, isolated data directory.
import { spawn, spawnSync } from "node:child_process";
import { mkdtemp, readFile, writeFile, mkdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { randomBytes } from "node:crypto";
import assert from "node:assert/strict";
import { zip } from "./zip.mjs";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const temporary = await mkdtemp(join(tmpdir(), "pier-smoke-"));
const port = 18099;
const base = `http://127.0.0.1:${port}`;
const wsl = process.argv.includes("--wsl");
const linuxPath = (path) =>
  path
    .replaceAll("\\", "/")
    .replace(/^([A-Za-z]):/, (_, d) => "/mnt/" + d.toLowerCase());
const executable = wsl
  ? "wsl"
  : resolve(
      root,
      `server/bin/pier${process.platform === "win32" ? ".exe" : ""}`,
    );
const args = wsl
  ? [
      "-d",
      "Ubuntu",
      "--",
      "env",
      `PIER_LISTEN=127.0.0.1:${port}`,
      `PIER_DATA_DIR=${linuxPath(temporary)}`,
      `PIER_PROJECTS_DIR=${linuxPath(join(temporary, "projects"))}`,
      "PIER_ADMIN_PASSWORD=",
      "PIER_ADMIN_USERNAME=",
      linuxPath(resolve(root, "server/bin/pier-linux-amd64")),
    ]
  : [];
const child = spawn(executable, args, {
  cwd: resolve(root, "server"),
  env: {
    ...process.env,
    PIER_LISTEN: `127.0.0.1:${port}`,
    PIER_DATA_DIR: temporary,
    PIER_PROJECTS_DIR: join(temporary, "projects"),
    PIER_ADMIN_PASSWORD: "",
    PIER_ADMIN_USERNAME: "",
  },
  windowsHide: true,
  stdio: ["ignore", "pipe", "pipe"],
});
let output = "";
child.stdout.on("data", (b) => {
  output += b;
});
child.stderr.on("data", (b) => {
  output += b;
});
let cookie = "";
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
async function api(path, data, method = "POST") {
  const response = await fetch(base + "/api/v1" + path, {
    method: data === undefined ? "GET" : method,
    headers: {
      Cookie: cookie,
      ...(data instanceof FormData
        ? {}
        : { "Content-Type": "application/json" }),
    },
    body:
      data === undefined
        ? undefined
        : data instanceof FormData
          ? data
          : JSON.stringify(data),
  });
  const value = await response.json();
  if (!response.ok)
    throw new Error(`${response.status}: ${JSON.stringify(value)}`);
  if (response.headers.get("set-cookie"))
    cookie = response.headers.get("set-cookie").split(";")[0];
  return value;
}
async function completed(id) {
  for (let i = 0; i < 100; i++) {
    const t = await api(`/tasks/${id}`);
    if (!["queued", "running"].includes(t.status)) return t;
    await wait(150);
  }
  throw new Error("Task timed out");
}
try {
  let ready = false;
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(base + "/healthz")).ok) {
        ready = true;
        break;
      }
    } catch {}
    await wait(100);
  }
  assert(ready, `Server did not start: ${output}`);
  assert.equal((await api("/auth/status")).initialized, false);
  const credentials = {
    username: "smoke-admin",
    password: randomBytes(24).toString("hex"),
  };
  await api("/auth/setup", credentials);
  await api("/auth/login", credentials);
  await api("/projects", {
    id: "hello-pier",
    name: "Smoke project",
    type: "static",
    configYaml: "bundles:\n  keep: 3\n",
  });
  const archive = await readFile(resolve(root, "examples/hello-pier.zip"));
  const form = new FormData();
  form.append("file", new Blob([archive]), "bundle.zip");
  const created = await api("/projects/hello-pier/upload", form);
  const result = await completed(created.id);
  const logs = (await api(`/tasks/${created.id}/logs`)).text;
  if (result.status !== "succeeded")
    throw new Error(`Deployment failed:\n${logs}`);
  assert.equal(
    (await api("/projects/hello-pier")).state.currentVersion,
    "v1.0.0",
  );
  const badVersion = zip([
    {
      name: "upy.yaml",
      data: `name: hello-pier\nversion: v1.2.0\ntype: static\nhealthcheck:\n  url: http://127.0.0.1:${port}/api/v1/does-not-exist\n  timeout: 1\n`,
    },
    { name: "dist/index.html", data: "<h1>unhealthy</h1>" },
  ]);
  const bad = new FormData();
  bad.append("file", new Blob([badVersion]), "v1.2.0.zip");
  const failed = await api("/projects/hello-pier/upload", bad);
  assert.equal((await completed(failed.id)).status, "failed");
  assert.equal(
    (await api("/projects/hello-pier")).state.currentVersion,
    "v1.0.0",
  );
  const failedLogs = (await api(`/tasks/${failed.id}/logs`)).text;
  assert(failedLogs.includes("已回滚到 v1.0.0"));
  if (wsl || process.platform !== "win32") {
    const projectRoot = join(temporary, "projects/hello-pier");
    await mkdir(join(projectRoot, "shared"), { recursive: true });
    await writeFile(
      join(projectRoot, "shared/data.txt"),
      "persistent application data",
    );
    await writeFile(
      join(projectRoot, "backup.sh"),
      '#!/bin/sh\nset -eu\nmkdir -p "$UPY_BACKUP_DIR"\ntar -czf "$UPY_BACKUP_DIR/smoke.tar.gz" -C "$UPY_PROJECT_ROOT" shared\n',
    );
    const backup = await api("/projects/hello-pier/tasks", { kind: "backup" });
    const backed = await completed(backup.id);
    if (backed.status !== "succeeded")
      throw new Error((await api(`/tasks/${backup.id}/logs`)).text);
    assert.equal((await api("/projects/hello-pier")).backups.length, 1);
  }
  assert(logs.includes("任务完成"));
  const version2 = zip([
    {
      name: "upy.yaml",
      data: "name: hello-pier\nversion: v1.1.0\ntype: static\n",
    },
    { name: "dist/index.html", data: "<h1>v1.1.0</h1>" },
  ]);
  const next = new FormData();
  next.append("file", new Blob([version2]), "v1.1.0.zip");
  const second = await api("/projects/hello-pier/upload", next);
  const secondResult = await completed(second.id);
  if (secondResult.status !== "succeeded")
    throw new Error((await api(`/tasks/${second.id}/logs`)).text);
  assert.equal(
    (await api("/projects/hello-pier")).state.currentVersion,
    "v1.1.0",
  );
  const rollback = await api("/projects/hello-pier/tasks", {
    kind: "rollback",
    target: "v1.0.0",
  });
  const rolled = await completed(rollback.id);
  if (rolled.status !== "succeeded")
    throw new Error((await api(`/tasks/${rollback.id}/logs`)).text);
  assert.equal(
    (await api("/projects/hello-pier")).state.currentVersion,
    "v1.0.0",
  );
  const clean = await api("/projects/hello-pier/tasks", {
    kind: "clean",
    target: "bundles",
  });
  assert.equal((await completed(clean.id)).status, "succeeded");
  assert.equal(
    (await api("/projects/hello-pier")).state.currentVersion,
    "v1.0.0",
  );
  console.log(
    "PASS: auth → upload v1 → deploy v2 → manual rollback → failed healthcheck auto rollback → backup → logs → safe cleanup",
  );
} finally {
  if (wsl) {
    const pid = await readFile(join(temporary, "pier.pid"), "utf8").catch(
      () => "",
    );
    if (/^\d+$/.test(pid))
      spawnSync("wsl", ["-d", "Ubuntu", "--", "kill", "-TERM", pid], {
        windowsHide: true,
      });
  } else child.kill("SIGTERM");
  await new Promise((r) => {
    if (child.exitCode !== null) r();
    else {
      child.once("exit", r);
      setTimeout(r, 3000).unref();
    }
  });
  const safe = resolve(temporary);
  if (
    safe.startsWith(resolve(tmpdir()) + sep) &&
    safe.split(sep).at(-1).startsWith("pier-smoke-")
  ) {
    // Windows cannot unlink WSL's Linux symlink reparse points.
    if (wsl)
      spawnSync(
        "wsl",
        [
          "-d",
          "Ubuntu",
          "--",
          "unlink",
          linuxPath(join(safe, "projects/hello-pier/current")),
        ],
        { windowsHide: true },
      );
    await rm(safe, { recursive: true, force: true });
  }
}
