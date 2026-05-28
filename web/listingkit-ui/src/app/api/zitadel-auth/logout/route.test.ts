import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
  signOutResult: new Response(null, { status: 302 }),
}));

const mockedZitadelHelpers = vi.hoisted(() => ({
  options: undefined as
    | {
        issuerUrl: string;
        clientId: string;
        clientSecret?: string;
        redirectUri?: string;
        postLogoutRedirectUri?: string;
        scopes: string;
      }
    | undefined,
  discovery: undefined as
    | {
        end_session_endpoint?: string;
      }
    | undefined,
  discoveryError: null as Error | null,
  publicOrigin: "http://localhost:3000",
}));

vi.mock("@/auth", () => ({
  auth: vi.fn(() => Promise.resolve(mockedAuthState.session)),
  signOut: vi.fn(() => Promise.resolve(mockedAuthState.signOutResult)),
}));

vi.mock("@/lib/server/zitadel-auth", () => ({
  getZitadelAuthOptions: vi.fn(() => mockedZitadelHelpers.options),
  fetchZitadelDiscovery: vi.fn(async () => {
    if (mockedZitadelHelpers.discoveryError) {
      throw mockedZitadelHelpers.discoveryError;
    }
    return mockedZitadelHelpers.discovery;
  }),
  readZitadelIDTokenFromSession: vi.fn((session) =>
    typeof session?.idToken === "string" ? session.idToken : "",
  ),
  resolvePublicAppOrigin: vi.fn(() => mockedZitadelHelpers.publicOrigin),
}));

import { GET } from "@/app/api/zitadel-auth/logout/route";
import { signOut } from "@/auth";

describe("GET /api/zitadel-auth/logout", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    mockedAuthState.session = null;
    mockedAuthState.signOutResult = new Response(null, { status: 302 });
    mockedZitadelHelpers.options = undefined;
    mockedZitadelHelpers.discovery = undefined;
    mockedZitadelHelpers.discoveryError = null;
    mockedZitadelHelpers.publicOrigin = "http://localhost:3000";
  });

  it("falls back to a local signout when OIDC discovery fails", async () => {
    mockedZitadelHelpers.options = {
      issuerUrl: "https://auth.shuomiai.com",
      clientId: "client-1",
      clientSecret: "secret-1",
      postLogoutRedirectUri: "http://localhost:3000",
      scopes: "openid profile",
    };
    mockedZitadelHelpers.discoveryError = new Error("fetch failed");

    await expect(GET()).resolves.toBe(mockedAuthState.signOutResult);

    expect(signOut).toHaveBeenCalledWith({
      redirectTo: "http://localhost:3000",
    });
  });
});
