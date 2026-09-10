import { spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { createWriteStream } from "node:fs";
import { access, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { encode } from "@auth/core/jwt";

const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(web, "../..");
const dir = await mkdtemp(join(tmpdir(), "issue370-"));
const children = [];
let containerId = "";
let goProcess;
let nextProcess;
let nextPort;
let closing;

function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const process = spawn(command, args, {
      cwd: repo,
      windowsHide: true,
      ...options,
    });
    let output = "";
    process.stdout?.on("data", (data) => (output += data));
    process.stderr?.on("data", (data) => (output += data));
    process.on("error", reject);
    process.on("exit", (code) =>
      code === 0
        ? resolveRun(output.trim())
        : reject(
            new Error(
              `${command} failed (${code}): ${output.slice(-4_000)}`,
            ),
          ),
    );
  });
}

function start(command, args, logName, options = {}) {
  const output = createWriteStream(join(dir, logName));
  const process = spawn(command, args, {
    cwd: repo,
    windowsHide: true,
    stdio: ["ignore", "pipe", "pipe"],
    ...options,
  });
  process.stdout.pipe(output);
  process.stderr.pipe(output);
  process.on("exit", () => output.end());
  children.push(process);
  return process;
}

function pnpmCommand(args) {
  return process.platform === "win32"
    ? { command: "cmd", args: ["/c", "pnpm.cmd", ...args] }
    : { command: "pnpm", args };
}

const pause = (milliseconds) =>
  new Promise((resolvePause) => setTimeout(resolvePause, milliseconds));

async function until(check, label, timeout = 120_000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    try {
      const value = await check();
      if (value) return value;
    } catch {
      // Startup probes are expected to fail until their owner is ready.
    }
    if (children.some((child) => child.exitCode !== null)) {
      throw new Error(`Fixture exited during ${label}; logs: ${dir}`);
    }
    await pause(250);
  }
  throw new Error(`Timed out waiting for ${label}; logs: ${dir}`);
}

async function freePort(preferred = 0) {
  const server = createServer();
  await new Promise((resolveListen, reject) => {
    server.once("error", reject);
    server.listen(preferred, "127.0.0.1", resolveListen);
  });
  const address = server.address();
  const port = typeof address === "object" && address ? address.port : 0;
  await new Promise((resolveClose) => server.close(resolveClose));
  return port;
}

async function stopChild(child) {
  if (!child || child.exitCode !== null) return;
  if (process.platform === "win32") {
    await run("taskkill", ["/PID", String(child.pid), "/T", "/F"]);
  } else {
    child.kill("SIGTERM");
  }
}

async function cleanup() {
  return (closing ??= (async () => {
    const errors = [];
    try {
      await stopChild(nextProcess);
    } catch (error) {
      errors.push(error);
    }
    try {
      if (nextPort) {
        await until(
          async () => (await freePort(nextPort)) === nextPort,
          "Next port release",
          10_000,
        );
      }
    } catch (error) {
      errors.push(error);
    }
    try {
      if (goProcess && goProcess.exitCode === null) {
        await writeFile(join(dir, "stop-go"), "stop");
        await Promise.race([
          new Promise((resolveExit) => goProcess.once("exit", resolveExit)),
          pause(10_000),
        ]);
      }
      if (goProcess && goProcess.exitCode === null) {
        await stopChild(goProcess);
      }
      if (goProcess && goProcess.exitCode !== 0) {
        errors.push(new Error(`Go fixture failed; inspect ${join(dir, "go.log")}`));
      }
    } catch (error) {
      errors.push(error);
    }
    try {
      if (containerId) await run("docker", ["stop", containerId]);
    } catch (error) {
      errors.push(error);
    }
    await writeFile(
      join(dir, "cleanup.json"),
      JSON.stringify({
        nextPid: nextProcess?.pid,
        nextPort,
        nextPortReleased: Boolean(nextPort),
        goExit: goProcess?.exitCode,
        postgresStopped: Boolean(containerId),
      }),
    );
    if (errors.length) throw new AggregateError(errors, "Fixture cleanup failed");
  })());
}

process.once("SIGINT", () => {
  void cleanup().then(() => process.exit(0), () => process.exit(1));
});
process.once("SIGTERM", () => {
  void cleanup().then(() => process.exit(0), () => process.exit(1));
});

try {
  await access(join(web, "node_modules/next/dist/bin/next"));
  const binary = join(
    dir,
    process.platform === "win32"
      ? "source-account.test.exe"
      : "source-account.test",
  );
  console.log(`Building the isolated Source Account fixture; evidence ${dir}`);
  await run("go", ["test", "-c", "-o", binary, "./internal/app/httpapi"]);

  const postgresPassword = randomBytes(24).toString("base64url");
  const containerName = `issue370-${randomUUID()}`;
  containerId = await run("docker", [
    "run",
    "--rm",
    "-d",
    "--name",
    containerName,
    "-e",
    `POSTGRES_PASSWORD=${postgresPassword}`,
    "-e",
    "POSTGRES_DB=issue370_fixture",
    "-p",
    "127.0.0.1::5432",
    "postgres:16-alpine",
  ]);
  const mapping = await run("docker", ["port", containerId, "5432/tcp"]);
  if (!/^127\.0\.0\.1:\d+$/.test(mapping)) {
    throw new Error(`Unexpected PostgreSQL mapping: ${mapping}`);
  }
  const postgresPort = Number(mapping.split(":").at(-1));
  await until(async () => {
    await run("docker", [
      "exec",
      containerId,
      "pg_isready",
      "-U",
      "postgres",
      "-d",
      "issue370_fixture",
    ]);
    return true;
  }, "task-owned PostgreSQL");

  const configPath = join(dir, "source-account.yaml");
  await writeFile(
    configPath,
    [
      "database:",
      '  host: "127.0.0.1"',
      `  port: ${postgresPort}`,
      '  user: "postgres"',
      `  password: "${postgresPassword}"`,
      '  database: "issue370_fixture"',
      "  max_connections: 10",
      "  max_idle_connections: 5",
      '  connection_max_lifetime: "1h"',
      "",
    ].join("\n"),
    { mode: 0o600 },
  );
  const schemaCommand = [
    "run",
    "./cmd/source-account-registry-schema-init",
    "-config",
    configPath,
  ];
  await run("go", schemaCommand);
  await run("go", schemaCommand);

  const dsn = `host=127.0.0.1 port=${postgresPort} user=postgres password=${postgresPassword} dbname=issue370_fixture sslmode=disable`;
  goProcess = start(
    binary,
    [
      "-test.run=^TestSourceAccountBrowserFixture$",
      "-test.v",
      "-test.timeout=30m",
    ],
    "go.log",
    {
      env: {
        ...process.env,
        ISSUE370_FIXTURE_DIR: dir,
        ISSUE370_FIXTURE_DSN: dsn,
      },
    },
  );
  const go = await until(
    async () => JSON.parse(await readFile(join(dir, "go.json"), "utf8")),
    "Source Account Go application",
  );

  nextPort = await freePort();
  const origin = `http://127.0.0.1:${nextPort}`;
  const authSecret = randomBytes(48).toString("base64url");
  nextProcess = start(
    process.execPath,
    [
      join(web, "node_modules/next/dist/bin/next"),
      "dev",
      "--hostname",
      "127.0.0.1",
      "--port",
      String(nextPort),
    ],
    "next.log",
    {
      cwd: web,
      env: {
        ...process.env,
        NODE_ENV: "development",
        NEXT_TELEMETRY_DISABLED: "1",
        AUTH_SECRET: authSecret,
        AUTH_URL: origin,
        AUTH_TRUST_HOST: "true",
        LISTINGKIT_PUBLIC_BASE_URL: origin,
        ZITADEL_ISSUER_URL: "http://127.0.0.1:1/fixture-external-identity",
        ZITADEL_CLIENT_ID: "fixture-client",
        ZITADEL_CLIENT_SECRET: "",
        LISTINGKIT_SERVICE_API_BASE: `${go.goOrigin}/api/v1`,
      },
    },
  );

  const roles = {
    "actor-a": "listingkit_operator",
    "actor-c": "listingkit_operator",
    viewer: "listingkit_viewer",
  };
  const sessions = {};
  for (const [name, accessToken] of Object.entries(go.tokens)) {
    const value = await encode({
      secret: authSecret,
      salt: "authjs.session-token",
      maxAge: 1_800,
      token: {
        sub: name,
        name: `Fixture ${name}`,
        accessToken,
        expiresAt: Math.floor(Date.now() / 1_000) + 1_800,
        identityVersion: 3,
        identity: {
          tenantId: "home-org",
          userId: name,
          roles: [roles[name]],
          userType: "zitadel",
        },
      },
    });
    sessions[name] = {
      subject: name,
      cookie: `authjs.session-token=${value}`,
    };
  }
  const sourceHead = await run("git", ["rev-parse", "HEAD"]);
  const manifestPath = join(dir, "fixture.json");
  const evidencePath = join(dir, "evidence.json");
  await writeFile(
    manifestPath,
    JSON.stringify(
      {
        origin,
        goOrigin: go.goOrigin,
        sourceHead,
        goHead: sourceHead,
        evidencePath,
        sessions,
      },
      null,
      2,
    ),
    { mode: 0o600 },
  );
  await until(
    async () => (await fetch(`${origin}/api/auth/session`)).ok,
    "Next/Auth.js server",
  );

  const pnpm = pnpmCommand([
    "exec",
    "vitest",
    "run",
    "--config",
    "e2e/issue370-source-account.config.ts",
  ]);
  const output = await run(pnpm.command, pnpm.args, {
    cwd: web,
    env: {
      ...process.env,
      SOURCE_ACCOUNT_FIXTURE_MANIFEST: manifestPath,
    },
  });
  console.log(output);
  console.log(
    `PASS actual Source Account client -> Next/Auth.js -> Go -> isolated PostgreSQL; ${evidencePath}`,
  );
} finally {
  await cleanup();
  console.log(
    `Owned Next, Go and PostgreSQL resources cleaned; evidence retained at ${dir}`,
  );
}
