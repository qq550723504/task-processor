import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getStoreProfiles,
} from "@/lib/api/store-profiles";

describe("store profile api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("reads store profiles", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ items: [{ id: 1, store_id: 869 }] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getStoreProfiles()).resolves.toEqual([{ id: 1, store_id: 869 }]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/store-profiles",
      expect.objectContaining({ method: "GET" }),
    );
  });

});
