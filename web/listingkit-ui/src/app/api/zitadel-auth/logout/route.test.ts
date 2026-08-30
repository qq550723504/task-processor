import { afterEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const mockedAuthState = vi.hoisted(() => ({
  signOutResult: new Response(null, { status: 302 }),
}));

const mockedServerToken = vi.hoisted(() => ({
  idToken: "",
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
  serverAuth: vi.fn(
    (handler: (request: NextRequest, context: unknown) => unknown) =>
      (request: NextRequest, context: unknown) =>
        handler(
          Object.assign(request, {
            auth: { idToken: mockedServerToken.idToken },
          }),
          context,
        ),
  ),
  signOut: vi.fn(() => Promise.resolve(mockedAuthState.signOutResult)),
}));

vi.mock("@/lib/server/zitadel-server-token", () => ({
  readZitadelServerIDToken: vi.fn(() => mockedServerToken.idToken),
}));

vi.mock("@/lib/server/zitadel-auth", () => ({
  getZitadelAuthOptions: vi.fn(() => mockedZitadelHelpers.options),
  fetchZitadelDiscovery: vi.fn(async () => {
    if (mockedZitadelHelpers.discoveryError) {
      throw mockedZitadelHelpers.discoveryError;
    }
    return mockedZitadelHelpers.discovery;
  }),
  resolvePublicAppOrigin: vi.fn(() => mockedZitadelHelpers.publicOrigin),
}));

import { GET } from "@/app/api/zitadel-auth/logout/route";
import { signOut } from "@/auth";

async function callGET() {
  const response = await GET(
    new NextRequest("http://localhost:3000/api/zitadel-auth/logout"),
    {} as never,
  );
  if (!(response instanceof Response)) {
    throw new Error("ZITADEL logout route did not return a response");
  }
  return response;
}

describe("GET /api/zitadel-auth/logout", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    mockedAuthState.signOutResult = new Response(null, { status: 302 });
    mockedServerToken.idToken = "";
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

    await expect(callGET()).resolves.toBe(mockedAuthState.signOutResult);

    expect(signOut).toHaveBeenCalledWith({
      redirectTo: "http://localhost:3000",
    });
  });

  it("uses only the encrypted server JWT ID token as the logout hint", async () => {
    mockedZitadelHelpers.options = {
      issuerUrl: "https://auth.shuomiai.com",
      clientId: "client-1",
      postLogoutRedirectUri: "http://localhost:3000",
      scopes: "openid profile",
    };
    mockedZitadelHelpers.discovery = {
      end_session_endpoint: "https://auth.shuomiai.com/oidc/v1/end_session",
    };
    mockedServerToken.idToken = "server-id-token";

    await callGET();

    expect(signOut).toHaveBeenCalledWith({
      redirectTo:
        "https://auth.shuomiai.com/oidc/v1/end_session?client_id=client-1&post_logout_redirect_uri=http%3A%2F%2Flocalhost%3A3000&id_token_hint=server-id-token",
    });
  });
});
