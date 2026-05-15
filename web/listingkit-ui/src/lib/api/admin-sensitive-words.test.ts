import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingSensitiveWord,
  getListingSensitiveWords,
  parseSensitiveWordPageResponse,
} from "@/lib/api/admin-sensitive-words";

describe("parseSensitiveWordPageResponse", () => {
  it("accepts the ListingKit sensitive word page shape", () => {
    expect(
      parseSensitiveWordPageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            word: "restricted",
            language: "en",
            tags: "policy",
            level: 2,
            replaceWord: "safe",
            status: 1,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, word: "restricted", language: "en" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseSensitiveWordPageResponse({ items: [{}], total: "1" }),
    ).toThrow(
      "ListingKit API returned an unexpected sensitive word page response",
    );
  });
});

describe("admin sensitive word API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests sensitive word pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingSensitiveWords({
        page: 2,
        page_size: 10,
        language: "en",
        word: "restricted",
      }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/sensitive-words?language=en&page=2&page_size=10&word=restricted",
    );
  });

  it("creates sensitive words", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          word: "restricted",
          language: "en",
          level: 2,
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingSensitiveWord({
        word: "restricted",
        language: "en",
        level: 2,
        status: 1,
      }),
    ).resolves.toMatchObject({ id: 1, word: "restricted" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/sensitive-words",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});
