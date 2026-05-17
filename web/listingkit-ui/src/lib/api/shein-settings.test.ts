import { afterEach, describe, expect, it, vi } from "vitest";

import { getSheinSettings, updateSheinSettings } from "@/lib/api/shein-settings";

describe("shein settings api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("reads shein settings through the namespace route", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ default_store_id: 869 }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getSheinSettings()).resolves.toEqual({ default_store_id: 869 });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/shein",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("updates shein settings through the namespace route", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ default_store_id: 869 }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await updateSheinSettings({ default_store_id: 869 });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/shein",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ default_store_id: 869 }),
      }),
    );
  });
});
