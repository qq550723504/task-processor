import { afterEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const mockedZitadelHelpers = vi.hoisted(() => ({
  options: {
    issuerUrl: "https://auth.shuomiai.com",
    clientId: "listingkit-client",
    scopes: "openid profile",
  } as
    | {
        issuerUrl: string;
        clientId: string;
        scopes: string;
      }
    | undefined,
}));

vi.mock("@/auth", () => ({
  signIn: vi.fn(() => Promise.resolve(new Response(null, { status: 302 }))),
}));

vi.mock("@/lib/server/zitadel-auth", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("@/lib/server/zitadel-auth")>();
  return {
    ...original,
    getZitadelAuthOptions: vi.fn(() => mockedZitadelHelpers.options),
  };
});

import { GET } from "@/app/api/zitadel-auth/login/route";
import { signIn } from "@/auth";

async function callGET(path: string) {
  const response = await GET(new NextRequest(`http://localhost:3000${path}`));
  if (!(response instanceof Response)) {
    throw new Error("ZITADEL login route did not return a response");
  }
  return response;
}

describe("GET /api/zitadel-auth/login", () => {
  afterEach(() => {
    vi.clearAllMocks();
    mockedZitadelHelpers.options = {
      issuerUrl: "https://auth.shuomiai.com",
      clientId: "listingkit-client",
      scopes: "openid profile",
    };
  });

  it("keeps bare login on the generic Auth.js ZITADEL provider", async () => {
    await callGET(
      "/api/zitadel-auth/login?returnTo=%2Fworkbench%2Fstores%3Ftab%3Dactive",
    );

    expect(signIn).toHaveBeenCalledWith("zitadel", {
      redirectTo: "/workbench/stores?tab=active",
    });
  });

  it.each(["otp", "password"])(
    "fails closed while the %s Login V2 capability is unverified",
    async (method) => {
      const response = await callGET(
        `/api/zitadel-auth/login?method=${method}&returnTo=%2Fworkbench`,
      );

      expect(response.status).toBe(503);
      await expect(response.json()).resolves.toEqual({
        error: "login_capability_unavailable",
        entry: `${method}_login`,
      });
      expect(signIn).not.toHaveBeenCalled();
    },
  );

  it.each([
    "magic-link",
    "__proto__",
    "otp&method=password",
  ])("does not let %s select a specialized upstream entry", async (method) => {
    await callGET(
      `/api/zitadel-auth/login?method=${method}&returnTo=%2Fworkbench`,
    );

    expect(signIn).toHaveBeenCalledWith("zitadel", {
      redirectTo: "/workbench",
    });
  });

  it("normalizes returnTo again at the Auth.js boundary", async () => {
    await callGET(
      "/api/zitadel-auth/login?returnTo=https%3A%2F%2Fevil.example%2Fphish",
    );

    expect(signIn).toHaveBeenCalledWith("zitadel", { redirectTo: "/" });
  });
});
