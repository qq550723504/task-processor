import { beforeEach, describe, expect, it, vi } from "vitest";

import { manualSDSLogin, triggerSDSLogin } from "@/lib/api/sds-login";

describe("SDS login API", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts automatic login in headless mode for cluster execution", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await triggerSDSLogin();

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/sds-login/login",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ force_login: true, headless: true }),
      }),
    );
  });

  it("starts manual login in headless mode for cluster execution", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await manualSDSLogin({
      tenantID: "1",
      identifier: "869",
      merchantName: "demo-shop",
      username: "demo-user",
      password: "password",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/sds-login/manual-login",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: "1",
          identifier: "869",
          merchant_name: "demo-shop",
          username: "demo-user",
          password: "password",
          force_login: true,
          headless: true,
        }),
      }),
    );
  });
});
