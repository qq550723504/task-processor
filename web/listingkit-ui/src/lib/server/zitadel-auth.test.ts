import { afterEach, describe, expect, it, vi } from "vitest";

import { getZitadelAuthOptions } from "@/lib/server/zitadel-auth";

describe("getZitadelAuthOptions", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("requests the ZITADEL resource owner scope by default for tenant identity", () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "http://localhost:8080");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    expect(getZitadelAuthOptions()?.scopes.split(/\s+/)).toContain(
      "urn:zitadel:iam:user:resourceowner",
    );
  });
});
