import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { GenerationTopicPolicyAdminPage } from "@/components/listingkit/admin/generation-topic-policy-admin-page";
import * as generationTopicPoliciesApi from "@/lib/api/admin-generation-topic-policies";
import * as generationTopicOverridesApi from "@/lib/api/admin-generation-topic-overrides";

describe("GenerationTopicPolicyAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit generation topic policies", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          platform: "shein",
          topicKey: "children",
          remark: "manual",
          status: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "children",
          priority: 10,
          promptDirectives: [
            "Do not mention children, babies, or age-specific users.",
          ],
          lexiconByLanguage: { en: ["child", "children"], zh: ["儿童"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: [
              "Do not mention children, babies, or age-specific users.",
            ],
            lexiconByLanguage: { en: ["child", "children"], zh: ["儿童"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "生成禁用主题" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("manual")).toBeInTheDocument();
    });
    expect(await screen.findByText("Topic Catalog")).toBeInTheDocument();
    expect(screen.getAllByText("child, children")).toHaveLength(2);
    expect(
      generationTopicPoliciesApi.getListingGenerationTopicPolicies,
    ).toHaveBeenCalled();
    expect(generationTopicOverridesApi.getListingGenerationTopicCatalog).toHaveBeenCalled();
    expect(screen.getByText("manual")).toBeInTheDocument();
  });
});
