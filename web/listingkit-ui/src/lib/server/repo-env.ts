import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

import { loadEnvConfig } from "@next/env";

let loaded = false;
let cachedRepoRoot: string | null = null;
let cachedEnvFileValues: Map<string, string> | null = null;

function findRepoRoot(startDir: string) {
  let current = startDir;
  while (true) {
    if (existsSync(path.join(current, "go.mod")) || existsSync(path.join(current, ".env"))) {
      return current;
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return startDir;
    }
    current = parent;
  }
}

function stripWrappingQuotes(value: string) {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function parseEnvFile(filePath: string) {
  const values = new Map<string, string>();
  if (!existsSync(filePath)) {
    return values;
  }

  const raw = readFileSync(filePath, "utf8");
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) {
      continue;
    }

    const match = trimmed.match(/^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/);
    if (!match) {
      continue;
    }

    const [, key, value] = match;
    values.set(key, stripWrappingQuotes(value));
  }

  return values;
}

export function getRepoRoot() {
  if (cachedRepoRoot) {
    return cachedRepoRoot;
  }

  cachedRepoRoot = findRepoRoot(process.cwd());
  return cachedRepoRoot;
}

export function ensureRepoEnvLoaded() {
  if (loaded) {
    return;
  }

  const repoRoot = getRepoRoot();
  loadEnvConfig(repoRoot);
  loaded = true;
}

export function getRepoEnvValue(key: string) {
  ensureRepoEnvLoaded();
  const processValue = process.env[key]?.trim();
  if (processValue) {
    return processValue;
  }

  if (!cachedEnvFileValues) {
    const repoRoot = getRepoRoot();
    cachedEnvFileValues = new Map([
      ...parseEnvFile(path.join(repoRoot, ".env")),
      ...parseEnvFile(path.join(repoRoot, ".env.local")),
    ]);
  }

  return cachedEnvFileValues.get(key)?.trim();
}
