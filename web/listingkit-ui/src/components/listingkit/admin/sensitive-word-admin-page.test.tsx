import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SensitiveWordAdminPage } from "@/components/listingkit/admin/sensitive-word-admin-page";
import * as adminSensitiveWordsApi from "@/lib/api/admin-sensitive-words";

describe("SensitiveWordAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit sensitive words", async () => {
    vi.spyOn(adminSensitiveWordsApi, "getListingSensitiveWords").mockResolvedValue({
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
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <SensitiveWordAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "敏感词" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("restricted")).toBeInTheDocument();
    });
    expect(screen.getByText("policy")).toBeInTheDocument();
    expect(screen.getByText("safe")).toBeInTheDocument();
  });
});
