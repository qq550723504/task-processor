import { afterEach, describe, expect, it, vi } from "vitest";

const mockedJWT = vi.hoisted(() => ({
  getToken: vi.fn(),
  secret: "auth-secret" as string | undefined,
}));

vi.mock("next-auth/jwt", () => ({
  getToken: mockedJWT.getToken,
}));

vi.mock("@/auth.config", () => ({
  getAuthJsSecret: vi.fn(() => mockedJWT.secret),
}));

import {
  readZitadelServerAccessToken,
  readZitadelServerIDToken,
} from "@/lib/server/zitadel-server-token";

describe("server-only ZITADEL tokens", () => {
  const request = new Request("http://localhost:3000/listing-kits");

  afterEach(() => {
    vi.clearAllMocks();
    mockedJWT.secret = "auth-secret";
  });

  it("reads access and ID tokens from the encrypted Auth.js JWT", async () => {
    mockedJWT.getToken.mockResolvedValue({
      accessToken: "server-access-token",
      idToken: "server-id-token",
    });

    await expect(readZitadelServerAccessToken(request as never)).resolves.toBe(
      "server-access-token",
    );
    await expect(readZitadelServerIDToken(request as never)).resolves.toBe(
      "server-id-token",
    );
    expect(mockedJWT.getToken).toHaveBeenCalledWith({
      req: request,
      secret: "auth-secret",
      secureCookie: false,
    });
  });

  it("selects the secure Auth.js session cookie used on HTTPS", async () => {
    const secureRequest = new Request("https://listingkit.example.com/listing-kits", {
      headers: {
        cookie: "__Secure-authjs.session-token.0=encrypted-part",
      },
    });
    mockedJWT.getToken.mockResolvedValue({ accessToken: "secure-access-token" });

    await expect(
      readZitadelServerAccessToken(secureRequest as never),
    ).resolves.toBe("secure-access-token");
    expect(mockedJWT.getToken).toHaveBeenCalledWith({
      req: secureRequest,
      secret: "auth-secret",
      secureCookie: true,
    });
  });

  it("fails closed when the Auth.js secret is unavailable", async () => {
    mockedJWT.secret = undefined;

    await expect(readZitadelServerAccessToken(request as never)).resolves.toBe("");
    await expect(readZitadelServerIDToken(request as never)).resolves.toBe("");
    expect(mockedJWT.getToken).not.toHaveBeenCalled();
  });
});
