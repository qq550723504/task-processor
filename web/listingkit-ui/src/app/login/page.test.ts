import { beforeEach, describe, expect, it, vi } from "vitest";

const redirectMock = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  redirect: redirectMock,
}));

import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    redirectMock.mockReset();
  });

  it("redirects to the ZITADEL login endpoint with a normalized returnTo", async () => {
    await LoginPage({
      searchParams: Promise.resolve({
        returnTo: "/listing-kits/shein?step=generate",
      }),
    });

    expect(redirectMock).toHaveBeenCalledWith(
      "/api/zitadel-auth/login?returnTo=%2Flisting-kits%2Fshein%3Fstep%3Dgenerate",
    );
  });

  it("falls back to the app root for invalid returnTo values", async () => {
    await LoginPage({
      searchParams: Promise.resolve({
        returnTo: "https://evil.example/phish",
      }),
    });

    expect(redirectMock).toHaveBeenCalledWith(
      "/api/zitadel-auth/login?returnTo=%2F",
    );
  });
});
