import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getStoreProfiles,
  getStoreRouting,
  updateStoreRouting,
} from "@/lib/api/store-profiles";

describe("store profile api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("reads store profiles", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify([{ id: 1, store_id: 869 }]), {
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

  it("reads store routing settings", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ selection_strategy: "priority" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getStoreRouting()).resolves.toEqual({ selection_strategy: "priority" });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/store-routing",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("updates store routing settings", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ selection_strategy: "priority" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await updateStoreRouting({ selection_strategy: "priority", fallback_store_id: 869 });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/store-routing",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ selection_strategy: "priority", fallback_store_id: 869 }),
      }),
    );
  });
});
