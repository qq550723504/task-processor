import { execFile as execFileCallback } from "node:child_process";
import { randomUUID } from "node:crypto";
import { lstat, mkdir, rename, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

const acceptanceTokenFileEnv = "LISTINGKIT_ACCEPTANCE_TOKEN_FILE";
const execFile = promisify(execFileCallback);

export async function persistZitadelAcceptanceToken(token: string) {
  const normalizedToken = token.trim();
  if (!normalizedToken) {
    throw new Error("Missing ZITADEL access token for local acceptance handoff");
  }

  const configuredPath = process.env[acceptanceTokenFileEnv]?.trim();
  if (!configuredPath) {
    return false;
  }

  const tokenPath = path.resolve(configuredPath);
  const normalizedPath = tokenPath.replaceAll(path.sep, "/").toLowerCase();
  if (
    !normalizedPath.includes("/.local/image-agent-acceptance/") ||
    path.basename(tokenPath).toLowerCase() !== "user-token.txt"
  ) {
    throw new Error(`${acceptanceTokenFileEnv} must point to .local/image-agent-acceptance/user-token.txt`);
  }

  const tokenDirectory = path.dirname(tokenPath);
  await mkdir(tokenDirectory, { recursive: true, mode: 0o700 });
  await rejectReparsePoints(tokenDirectory);
  await rejectReparsePoints(tokenPath);

  const temporaryPath = path.join(
    tokenDirectory,
    `.${path.basename(tokenPath)}.${process.pid}.${randomUUID()}.tmp`,
  );
  try {
    await writeFile(temporaryPath, `${normalizedToken}\n`, {
      encoding: "utf8",
      flag: "wx",
      mode: 0o600,
    });
    if (process.platform === "win32") {
      await protectWindowsFile(temporaryPath);
    }
    await rejectReparsePoints(tokenDirectory);
    await rejectReparsePoints(tokenPath);
    await rm(tokenPath, { force: true });
    await rename(temporaryPath, tokenPath);
  } finally {
    await rm(temporaryPath, { force: true });
  }
  return true;
}

async function rejectReparsePoints(targetPath: string) {
  let current = path.resolve(targetPath);
  while (true) {
    try {
      const stat = await lstat(current);
      if (stat.isSymbolicLink()) {
        throw new Error("acceptance token path must not contain a symbolic link or reparse point");
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") {
        throw error;
      }
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return;
    }
    current = parent;
  }
}

async function protectWindowsFile(filePath: string) {
  const { stdout: whoamiOutput } = await execFile("whoami.exe", [
    "/user",
    "/fo",
    "csv",
    "/nh",
  ]);
  const currentSID = whoamiOutput.match(/S-\d-(?:\d+-)+\d+/)?.[0];
  if (!currentSID) {
    throw new Error("cannot resolve the current Windows user SID");
  }
  await execFile("icacls.exe", [
    filePath,
    "/inheritance:r",
    "/grant:r",
    `*${currentSID}:(F)`,
    "*S-1-5-18:(F)",
    "*S-1-5-32-544:(F)",
  ]);
  const { stdout: sddlOutput } = await execFile(
    "pwsh.exe",
    [
      "-NoProfile",
      "-NonInteractive",
      "-Command",
      "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); (Get-Acl -LiteralPath $env:LISTINGKIT_ACL_VERIFY_PATH).Sddl",
    ],
    {
      env: { ...process.env, LISTINGKIT_ACL_VERIFY_PATH: filePath },
    },
  );
  const sddl = sddlOutput.trim();
  if (!sddl.includes("D:P")) {
    throw new Error("acceptance token ACL inheritance is not disabled");
  }
  const allowed = new Set([
    currentSID,
    "SY",
    "BA",
    "S-1-5-18",
    "S-1-5-32-544",
  ]);
  const identities = [...sddl.matchAll(/;;;([^\)]+)\)/g)].map((match) => match[1]);
  if (identities.length === 0 || identities.some((identity) => !allowed.has(identity))) {
    throw new Error("acceptance token ACL contains an unexpected principal");
  }
}
