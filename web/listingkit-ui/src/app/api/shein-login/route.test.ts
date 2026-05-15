import { describe, expect, it } from "vitest";

import { buildSheinLoginUpstreamHeaders } from "@/app/api/shein-login/[...path]/route";

describe("buildSheinLoginUpstreamHeaders", () => {
  it("forwards only generic request headers", () => {
    const headers = buildSheinLoginUpstreamHeaders(
      new Headers({
        accept: "application/json",
        authorization: "Bearer token-1",
        "tenant-id": "286",
        "visit-tenant-id": "389",
        "login-user": encodeURIComponent(JSON.stringify({ id: 42, tenantId: 286 })),
      }),
    );

    expect(headers.get("Authorization")).toBe("Bearer token-1");
    expect(headers.get("tenant-id")).toBeNull();
    expect(headers.get("visit-tenant-id")).toBeNull();
    expect(headers.get("login-user")).toBeNull();
  });

  it("does not add a fallback tenant header when the request has no tenant context", () => {
    const headers = buildSheinLoginUpstreamHeaders(
      new Headers({
        accept: "application/json",
      }),
    );

    expect(headers.get("tenant-id")).toBeNull();
    expect(headers.get("visit-tenant-id")).toBeNull();
    expect(headers.get("login-user")).toBeNull();
  });
});
