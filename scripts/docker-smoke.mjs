// Tests the actual image, using disposable named volumes (no host Docker socket).
import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import assert from "node:assert/strict";
import { zip } from "./zip.mjs";

const image = process.argv[2] || "pier:smoke";
const suffix = randomBytes(6).toString("hex");
const container = `pier-smoke-${suffix}`;
const volume = `${container}-data`;
let cookie = "";
let base = "";
const credentials = {
  username: "smoke-admin",
  password: randomBytes(24).toString("hex"),
};
function docker(args, allowFailure = false) {
  const result = spawnSync("docker", args, {
    encoding: "utf8",
    windowsHide: true,
  });
  if (!allowFailure && result.status !== 0)
    throw new Error(result.stderr || result.error?.message || "Docker failed");
  return (result.stdout || "").trim();
}
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function ready() {
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(`${base}/healthz`)).ok) return;
    } catch {}
    await sleep(150);
  }
  throw new Error(
    `Container did not become ready:\n${docker(["logs", container], true)}`,
  );
}
async function api(path, data) {
  const response = await fetch(`${base}/api/v1${path}`, {
    method: data === undefined ? "GET" : "POST",
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
  const body = await response.json();
  if (!response.ok)
    throw new Error(`${response.status}: ${JSON.stringify(body)}`);
  const session = response.headers.get("set-cookie");
  if (session) cookie = session.split(";")[0];
  return body;
}
async function finish(id) {
  for (let i = 0; i < 100; i++) {
    const task = await api(`/tasks/${id}`);
    if (!["running", "queued"].includes(task.status)) {
      if (task.status !== "succeeded")
        throw new Error((await api(`/tasks/${id}/logs`)).text);
      return;
    }
    await sleep(150);
  }
  throw new Error("Deployment timed out");
}
function start() {
  docker([
    "run",
    "-d",
    "--name",
    container,
    "-p",
    "127.0.0.1::8099",
    "--mount",
    `type=volume,source=${volume},target=/data/pier`,
    image,
  ]);
  const port = docker(["port", container, "8099/tcp"]).split(":").at(-1);
  base = `http://127.0.0.1:${port}`;
}
try {
  docker(["volume", "create", volume]);
  start();
  await ready();
  assert.equal((await api("/auth/status")).initialized, false);
  await api("/auth/setup", credentials);
  await api("/auth/login", credentials);
  await api("/projects", {
    id: "image-demo",
    name: "Image smoke",
    type: "static",
  });
  const archive = zip([
    {
      name: "upy.yaml",
      data: "name: image-demo\nversion: v1.0.0\ntype: static\n",
    },
    {
      name: "dist/index.html",
      data: "<h1>Pier container deploy succeeded</h1>",
    },
  ]);
  const form = new FormData();
  form.append("file", new Blob([archive]), "bundle.zip");
  const task = await api("/projects/image-demo/upload", form);
  await finish(task.id);
  assert.equal(
    (await api("/projects/image-demo")).state.currentVersion,
    "v1.0.0",
  );
  assert(
    docker([
      "exec",
      container,
      "cat",
      "/data/pier/apps/image-demo/current/index.html",
    ]).includes("succeeded"),
  );
  docker(["exec", container, "docker", "--version"]);
  docker(["exec", container, "docker", "compose", "version"]);
  docker(["exec", container, "pier-crypt", "--version"]);
  docker([
    "exec",
    container,
    "wget",
    "-q",
    "--spider",
    "http://127.0.0.1:8099/healthz",
  ]);
  // Recreate the container: administrator, project, release, symlink and logs persist.
  docker(["stop", "--time", "15", container]);
  docker(["rm", container]);
  cookie = "";
  start();
  await ready();
  assert.equal((await api("/auth/status")).initialized, true);
  await api("/auth/login", credentials);
  assert.equal(
    (await api("/projects/image-demo")).state.currentVersion,
    "v1.0.0",
  );
  assert((await api(`/tasks/${task.id}/logs`)).text.includes("任务完成"));
  assert(
    docker([
      "exec",
      container,
      "cat",
      "/data/pier/apps/image-demo/current/index.html",
    ]).includes("succeeded"),
  );
  assert(
    (await fetch(base + "/")).headers
      .get("content-type")
      ?.includes("text/html"),
  );
  console.log(
    "PASS: image startup, login, embedded UI, ZIP deployment, Docker/Compose CLI, healthcheck and persistence after recreation",
  );
} finally {
  // Only the uniquely named resources created by this test are removed.
  docker(["stop", "--time", "15", container], true);
  docker(["rm", "-f", container], true);
  docker(["volume", "rm", volume], true);
}
