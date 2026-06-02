import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getListingGenerationTopicCatalog,
  parseGenerationTopicCatalogResponse,
} from "@/lib/api/admin-generation-topic-overrides";

describe("parseGenerationTopicCatalogResponse", () => {
  it("accepts the ListingKit generation topic catalog shape", () => {
    expect(
      parseGenerationTopicCatalogResponse({
        items: [
          {
            key: "children",
            priority: 10,
            promptDirectives: [
              "Do not mention children, babies, or age-specific users.",
            ],
            lexiconByLanguage: { en: ["child"], zh: ["儿童"] },
            tenantOverride: {
              status: 1,
              additionalPromptDirectives: ["Avoid toddler-focused positioning."],
              additionalLexiconByLanguage: { en: ["toddler"] },
            },
            effectiveDefinition: {
              promptDirectives: [
                "Do not mention children, babies, or age-specific users.",
                "Avoid toddler-focused positioning.",
              ],
              lexiconByLanguage: { en: ["child", "toddler"], zh: ["儿童"] },
            },
          },
        ],
      }),
    ).toMatchObject({
      items: [
        {
          key: "children",
          effectiveDefinition: {
            lexiconByLanguage: { en: ["child", "toddler"] },
          },
        },
      ],
    });
  });

  it("rejects invalid catalog payloads", () => {
    expect(() =>
      parseGenerationTopicCatalogResponse({ items: [{ key: 1 }] }),
    ).toThrow(
      "ListingKit API returned an unexpected generation topic catalog response",
    );
  });
});

describe("admin generation topic catalog API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests generation topic catalog through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          items: [
            {
              key: "children",
              priority: 10,
              promptDirectives: [],
              lexiconByLanguage: {},
              tenantOverride: null,
              effectiveDefinition: {
                promptDirectives: [],
                lexiconByLanguage: {},
              },
            },
          ],
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingGenerationTopicCatalog({ platform: "shein" }),
    ).resolves.toMatchObject({
      items: [{ key: "children" }],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/generation-topic-catalog?platform=shein",
    );
  });
});
