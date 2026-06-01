import { readFile } from "node:fs/promises";
import path from "node:path";
import packageJson from "../../package.json";

export type AppVersionInfo = {
  appVersion: string;
  buildId: string;
};

async function readNextBuildId() {
  try {
    const buildIdPath = path.join(process.cwd(), ".next", "BUILD_ID");
    return (await readFile(buildIdPath, "utf8")).trim();
  } catch {
    return "";
  }
}

export async function readAppVersionInfo(): Promise<AppVersionInfo> {
  const appVersion = packageJson.version;
  const buildId = (await readNextBuildId()) || appVersion;

  return {
    appVersion,
    buildId,
  };
}
