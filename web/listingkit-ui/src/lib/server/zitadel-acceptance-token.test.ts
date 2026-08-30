import { execFile as execFileCallback } from "node:child_process";
import { mkdtemp, mkdir, readFile, rm, symlink } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

import { afterEach, describe, expect, it, vi } from "vitest";

import { persistZitadelAcceptanceToken } from "@/lib/server/zitadel-acceptance-token";

const execFile = promisify(execFileCallback);
const roots: string[] = [];

describe("persistZitadelAcceptanceToken", () => {
  afterEach(async () => {
    vi.unstubAllEnvs();
    await Promise.all(roots.splice(0).map((root) => rm(root, { recursive: true, force: true })));
  });

  it("writes the token only to a protected acceptance file", async () => {
    const tokenPath = await acceptanceTokenPath();
    vi.stubEnv("LISTINGKIT_ACCEPTANCE_TOKEN_FILE", tokenPath);

    await expect(persistZitadelAcceptanceToken("access-token")).resolves.toBe(true);
    await expect(readFile(tokenPath, "utf8")).resolves.toBe("access-token\n");

    if (process.platform === "win32") {
      const { stdout } = await execFile(
        "pwsh.exe",
        [
          "-NoProfile",
          "-NonInteractive",
          "-Command",
          "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); (Get-Acl -LiteralPath $env:LISTINGKIT_ACL_VERIFY_PATH).Sddl",
        ],
        { env: { ...process.env, LISTINGKIT_ACL_VERIFY_PATH: tokenPath } },
      );
      expect(stdout.trim()).toMatch(/D:P/);
    }
  });

  it("rejects a pre-existing symbolic-link token target", async () => {
    const tokenPath = await acceptanceTokenPath();
    const outsidePath = path.join(path.dirname(path.dirname(tokenPath)), "outside-token.txt");
    await mkdir(path.dirname(tokenPath), { recursive: true });
    try {
      await symlink(outsidePath, tokenPath, "file");
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "EPERM") {
        return;
      }
      throw error;
    }
    vi.stubEnv("LISTINGKIT_ACCEPTANCE_TOKEN_FILE", tokenPath);

    await expect(persistZitadelAcceptanceToken("access-token")).rejects.toThrow(
      /symbolic link|reparse point/i,
    );
  });
});

async function acceptanceTokenPath() {
  const root = await mkdtemp(path.join(os.tmpdir(), "listingkit-token-test-"));
  roots.push(root);
  return path.join(root, ".local", "image-agent-acceptance", "user-token.txt");
}
