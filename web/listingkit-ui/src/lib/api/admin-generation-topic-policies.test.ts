import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingGenerationTopicPolicy,
  getListingGenerationTopicPolicies,
  parseGenerationTopicPolicyPageResponse,
} from "@/lib/api/admin-generation-topic-policies";

describe("parseGenerationTopicPolicyPageResponse", () => {
  it("accepts the ListingKit generation topic policy page shape", () => {
    expect(
      parseGenerationTopicPolicyPageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            platform: "shein",
            topicKey: "children",
            status: 1,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, platform: "shein", topicKey: "children" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseGenerationTopicPolicyPageResponse({ items: [{}], total: "1" }),
    ).toThrow(
      "ListingKit API returned an unexpected generation topic policy page response",
    );
  });
});

describe("admin generation topic policy API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests policy pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingGenerationTopicPolicies({
        page: 2,
        page_size: 10,
        platform: "shein",
        topic_key: "children",
      }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/generation-topic-policies?page=2&page_size=10&platform=shein&topic_key=children",
    );
  });

  it("creates generation topic policies", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          platform: "shein",
          topicKey: "children",
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingGenerationTopicPolicy({
        platform: "shein",
        topicKey: "children",
        status: 1,
      }),
    ).resolves.toMatchObject({ id: 1, topicKey: "children" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/generation-topic-policies",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});
