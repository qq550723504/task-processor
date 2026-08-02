import { beforeEach, describe, expect, it, vi } from "vitest";

import { cancelSheinLogin, loginSheinAccount } from "@/lib/api/shein-login";

describe("shein login api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts manual login in headless mode for cluster execution", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await loginSheinAccount(12);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/shein-login/accounts/12/login",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ force_login: true, headless: true }),
      }),
    );
  });

  it("cancels verify-code wait for a tenant-scoped account", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await cancelSheinLogin(870, "227");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/shein-login/accounts/870/verify-code-wait?tenant_id=227",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
