import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getPromptTemplateSchema,
  listPromptTemplateCatalog,
  listPromptTemplates,
  setPromptTemplateStatus,
  upsertPromptTemplate,
} from "@/lib/api/prompt-management";

describe("prompt management api", () => {
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

    await expect(listPromptTemplates()).resolves.toEqual({
      items: [{ key: "tmpl.key" }],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/prompts",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("lists prompt template catalog", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ items: [{ key: "tmpl.key" }] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listPromptTemplateCatalog()).resolves.toEqual({
      items: [{ key: "tmpl.key" }],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/prompts/catalog",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("reads a prompt template schema", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ key: "tmpl.key", category: "shein" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getPromptTemplateSchema("tmpl.key")).resolves.toEqual({
      key: "tmpl.key",
      category: "shein",
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/prompts/schema/tmpl.key",
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

    await upsertPromptTemplate({
      key: "tmpl.key",
      content: "hello",
      version: "v1",
      enabled: true,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/prompts",
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

    await setPromptTemplateStatus("tmpl.key", false);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/listing-kits/prompts/tmpl.key/status",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ enabled: false }),
      }),
    );
  });
});
