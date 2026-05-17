import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getListingKitSettingsSchema,
  listListingKitSettingsNamespaces,
} from "@/lib/api/listingkit-settings";

describe("listingkit settings namespace api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("lists settings namespaces", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ items: [{ namespace: "ai", label: "AI 配置" }] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listListingKitSettingsNamespaces()).resolves.toEqual({
      items: [{ namespace: "ai", label: "AI 配置" }],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("gets a namespace schema", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ namespace: "ai", label: "AI 配置" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getListingKitSettingsSchema("ai")).resolves.toEqual({
      namespace: "ai",
      label: "AI 配置",
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/ai/schema",
      expect.objectContaining({ method: "GET" }),
    );
  });
});
