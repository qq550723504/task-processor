import { describe, expect, it } from "vitest";

import { proxyAccountIdentity } from "./account-proxy";

describe("proxyAccountIdentity", () => {
  it("returns an identity-context error when the expected user is stale", async () => {
    const response = await proxyAccountIdentity(
      new Request("http://localhost/api/account/identity/profile", {
        method: "GET",
        headers: { "X-Expected-User-ID": "different-user" },
      }),
      "access-token",
      "session-user",
      "profile",
    );

    expect(response.status).toBe(409);
    await expect(response.json()).resolves.toMatchObject({ code: "IDENTITY_CONTEXT_CHANGED" });
  });

  it("keeps invalid methods as request errors", async () => {
    const response = await proxyAccountIdentity(
      new Request("http://localhost/api/account/identity/profile", {
        method: "POST",
        headers: { "X-Expected-User-ID": "different-user", "Content-Type": "application/json" },
        body: "{}",
      }),
      "access-token",
      "session-user",
      "profile",
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toMatchObject({ code: "INVALID_REQUEST" });
  });
});
