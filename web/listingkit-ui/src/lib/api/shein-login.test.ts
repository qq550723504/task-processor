import { beforeEach, describe, expect, it, vi } from "vitest";

import { loginSheinAccount } from "@/lib/api/shein-login";

describe("shein login api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts manual login in a visible browser for debugging", async () => {
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
        body: JSON.stringify({ force_login: true, headless: false }),
      }),
    );
  });
});
