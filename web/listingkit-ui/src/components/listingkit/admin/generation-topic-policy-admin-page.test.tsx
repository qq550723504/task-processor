import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { GenerationTopicPolicyAdminPage } from "@/components/listingkit/admin/generation-topic-policy-admin-page";
import * as generationTopicPoliciesApi from "@/lib/api/admin-generation-topic-policies";

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
    expect(
      generationTopicPoliciesApi.getListingGenerationTopicPolicies,
    ).toHaveBeenCalled();
    expect(screen.getByText("manual")).toBeInTheDocument();
  });
});
