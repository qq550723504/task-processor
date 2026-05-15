import { afterEach, describe, expect, it, vi } from "vitest";

import {
  listPromptSettings,
  setPromptSettingStatus,
  upsertPromptSetting,
} from "@/lib/api/prompt-settings";

describe("prompt settings api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("lists tenant prompt settings", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ items: [{ key: "tmpl.key" }] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listPromptSettings()).resolves.toEqual({
      items: [{ key: "tmpl.key" }],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/prompts",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("saves a prompt template", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ key: "tmpl.key", content: "hello" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await upsertPromptSetting({
      key: "tmpl.key",
      content: "hello",
      version: "v1",
      enabled: true,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/prompts",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          key: "tmpl.key",
          content: "hello",
          version: "v1",
          enabled: true,
        }),
      }),
    );
  });

  it("updates prompt status", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ key: "tmpl.key", enabled: false }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await setPromptSettingStatus("tmpl.key", false);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/prompts/tmpl.key/status",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ enabled: false }),
      }),
    );
  });
});
