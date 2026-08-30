import { describe, expect, it } from "vitest";

import {
  readZitadelServerAccessToken,
  readZitadelServerIDToken,
} from "@/lib/server/zitadel-server-token";

describe("server-only ZITADEL tokens", () => {
  it("reads the refreshed tokens carried by the server Auth.js session", () => {
    const session = {
      user: {},
      expires: new Date(Date.now() + 60_000).toISOString(),
      accessToken: "refreshed-access-token",
      idToken: "refreshed-id-token",
    };

    expect(readZitadelServerAccessToken(session)).toBe(
      "refreshed-access-token",
    );
    expect(readZitadelServerIDToken(session)).toBe("refreshed-id-token");
  });

  it("fails closed for absent or malformed server-session tokens", () => {
    expect(readZitadelServerAccessToken(null)).toBe("");
    expect(readZitadelServerIDToken(null)).toBe("");
    expect(
      readZitadelServerAccessToken({ accessToken: 42 } as never),
    ).toBe("");
    expect(readZitadelServerIDToken({ idToken: {} } as never)).toBe("");
  });
});
