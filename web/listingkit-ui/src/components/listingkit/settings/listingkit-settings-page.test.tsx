import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ListingKitSettingsPage } from "@/components/listingkit/settings/listingkit-settings-page";

vi.mock("@/components/listingkit/settings/zitadel-session-card", () => ({
  ZitadelSessionCard: () => <div>Zitadel card</div>,
}));

vi.mock("@/components/listingkit/settings/ai-client-settings-card", () => ({
  AIClientSettingsCard: () => <div>AI card</div>,
}));

vi.mock("@/components/listingkit/shein/shein-settings-card", () => ({
  SheinSettingsCard: () => <div>SHEIN card</div>,
}));

describe("ListingKitSettingsPage", () => {
  it("does not render prompt management in settings", () => {
    render(<ListingKitSettingsPage />);

    expect(screen.getByText("ListingKit 设置")).toBeInTheDocument();
    expect(screen.queryByText("当前客户提示词模板")).not.toBeInTheDocument();
  });
});
