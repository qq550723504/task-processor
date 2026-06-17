import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SettingsHealthCard } from "@/components/listingkit/settings/settings-health-card";
import { getListingKitSettingsHealth } from "@/lib/api/listingkit-settings";

vi.mock("@/lib/api/listingkit-settings", () => ({
  getListingKitSettingsHealth: vi.fn(),
}));

const mockedGetHealth = vi.mocked(getListingKitSettingsHealth);

describe("SettingsHealthCard", () => {
  it("renders blocked and unknown configuration impacts", async () => {
    mockedGetHealth.mockResolvedValue({
      status: "blocked",
      items: [
        {
          key: "ai.default",
          label: "AI 文案模型",
          status: "blocked",
          message: "model 缺失、api key 缺失",
          impact: ["生成 ListingKit 草稿"],
          action: "补齐 endpoint、model、api key。",
        },
        {
          key: "sds.session",
          label: "SDS 登录态",
          status: "unknown",
          message: "当前设置服务尚未接入该运行时探针，无法确认配置是否可用。",
          impact: ["SDS 属性补全"],
          action: "接入 SDS 登录态探针。",
        },
      ],
    });

    renderWithQueryClient(<SettingsHealthCard />);

    expect(screen.getByText("配置健康检查")).toBeInTheDocument();
    expect(await screen.findAllByText("存在阻断项")).toHaveLength(2);
    expect(screen.getByText("AI 文案模型")).toBeInTheDocument();
    expect(screen.getByText("model 缺失、api key 缺失")).toBeInTheDocument();
    expect(screen.getByText("影响：生成 ListingKit 草稿")).toBeInTheDocument();
    expect(screen.getByText("SDS 登录态")).toBeInTheDocument();
    expect(screen.getByText("待接入探针")).toBeInTheDocument();
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}
