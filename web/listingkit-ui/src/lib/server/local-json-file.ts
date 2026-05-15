import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";

import { resolveListingKitUILocalStoragePath } from "@/lib/server/local-storage-path";

export function readLocalJsonFileSync<T>(fileName: string, fallback: T): T {
  const storagePath = resolveListingKitUILocalStoragePath(fileName);
  if (!existsSync(storagePath)) {
    return fallback;
  }

  try {
    return JSON.parse(readFileSync(storagePath, "utf8")) as T;
  } catch {
    return fallback;
  }
}

export function writeLocalJsonFileSync(fileName: string, data: unknown) {
  const storagePath = resolveListingKitUILocalStoragePath(fileName);
  mkdirSync(path.dirname(storagePath), { recursive: true });
  const payload = `${JSON.stringify(data, null, 2)}\n`;
  const tempPath = `${storagePath}.${process.pid}.${Date.now()}.${Math.random()
    .toString(16)
    .slice(2)}.tmp`;
  writeFileSync(tempPath, payload, "utf8");
  renameSync(tempPath, storagePath);
}

export async function readLocalJsonFile<T>(fileName: string, fallback: T): Promise<T> {
  const storagePath = resolveListingKitUILocalStoragePath(fileName);

  try {
    return JSON.parse(await readFile(storagePath, "utf8")) as T;
  } catch {
    return fallback;
  }
}

export async function writeLocalJsonFile(fileName: string, data: unknown) {
  const storagePath = resolveListingKitUILocalStoragePath(fileName);
  await mkdir(path.dirname(storagePath), { recursive: true });
  const payload = `${JSON.stringify(data, null, 2)}\n`;
  const tempPath = `${storagePath}.${process.pid}.${Date.now()}.${Math.random()
    .toString(16)
    .slice(2)}.tmp`;
  await writeFile(tempPath, payload, "utf8");
  await rename(tempPath, storagePath);
}
