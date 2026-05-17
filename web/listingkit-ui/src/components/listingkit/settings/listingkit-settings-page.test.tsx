import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ListingKitSettingsPage } from "@/components/listingkit/settings/listingkit-settings-page";

vi.mock("@/components/listingkit/settings/zitadel-session-card", () => ({
  ZitadelSessionCard: () => <div>Zitadel card</div>,
}));

vi.mock("@/components/listingkit/settings/ai-client-settings-card", () => ({
  AIClientSettingsCard: () => <div>AI card</div>,
}));

vi.mock("@/lib/api/listingkit-settings", () => ({
  listListingKitSettingsNamespaces: vi.fn().mockResolvedValue({
    items: [
      {
        namespace: "ai",
        label: "AI 模型",
        description: "租户级和用户级模型配置。",
        supported_scopes: [{ id: "tenant", label: "租户" }, { id: "user", label: "用户" }],
      },
    ],
  }),
  getListingKitSettingsSchema: vi.fn(),
}));

describe("ListingKitSettingsPage", () => {
  it("renders metadata-backed settings sections without prompt or shein settings", async () => {
    renderWithQueryClient(<ListingKitSettingsPage />);

    expect(screen.getByText("ListingKit 设置")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "会话" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "AI 模型" })).toBeInTheDocument();
    expect(screen.getByTestId("settings-section-session")).toBeInTheDocument();
    expect(screen.getByTestId("settings-section-ai")).toBeInTheDocument();
    expect(screen.getByText("租户级和用户级模型配置。")).toBeInTheDocument();
    expect(screen.getByText("租户")).toBeInTheDocument();
    expect(screen.getByText("用户")).toBeInTheDocument();
    expect(screen.queryByText("当前客户提示词模板")).not.toBeInTheDocument();
    expect(screen.queryByText("店铺与价格规则")).not.toBeInTheDocument();
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}
