import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getAIClientSettings,
  updateAIClientSettings,
} from "@/lib/api/ai-client-settings";

describe("ai client settings api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("reads AI settings through the namespace route", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ client_name: "default", api_key_set: true }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getAIClientSettings("tenant", "default")).resolves.toEqual({
      client_name: "default",
      api_key_set: true,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(
        /^\/api\/listing-kits\/settings\/ai\?(?:client_name=default&scope=tenant|scope=tenant&client_name=default)$/,
      ),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("updates AI settings through the namespace route", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ client_name: "default", api_key_set: true }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await updateAIClientSettings({
      scope: "tenant",
      client_name: "default",
      base_url: "https://api.openai.com/v1",
      model: "gpt-4.1",
      timeout_second: 60,
      enabled: true,
      api_key: "secret",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/settings/ai",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          scope: "tenant",
          client_name: "default",
          base_url: "https://api.openai.com/v1",
          model: "gpt-4.1",
          timeout_second: 60,
          enabled: true,
          api_key: "secret",
        }),
      }),
    );
  });
});
