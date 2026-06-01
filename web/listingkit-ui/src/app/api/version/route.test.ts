import { afterEach, describe, expect, it, vi } from "vitest";

const mockedVersionState = vi.hoisted(() => ({
  appVersion: "0.1.0",
  buildId: "build-local",
}));

vi.mock("@/lib/app-version", () => ({
  readAppVersionInfo: vi.fn(async () => ({
    appVersion: mockedVersionState.appVersion,
    buildId: mockedVersionState.buildId,
  })),
}));

import { GET } from "@/app/api/version/route";

describe("GET /api/version", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    mockedVersionState.appVersion = "0.1.0";
    mockedVersionState.buildId = "build-local";
  });

  it("returns the current app version payload without allowing caches", async () => {
    mockedVersionState.appVersion = "0.2.0";
    mockedVersionState.buildId = "build-20260531";

    const response = await GET();
    const payload = (await response.json()) as {
      appVersion: string;
      buildId: string;
    };

    expect(response.status).toBe(200);
    expect(payload).toEqual({
      appVersion: "0.2.0",
      buildId: "build-20260531",
    });
    expect(response.headers.get("cache-control")).toContain("no-store");
  });
});
