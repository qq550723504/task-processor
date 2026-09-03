import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const context = vi.hoisted(() => ({
  isLoading: false,
  effectiveOrganization: { id: "org-a", name: "企业 A", roles: [] as string[] },
  roles: ["listingkit_operator"],
}));

vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => context,
}));
vi.mock("@/components/workbench/stores/store-form", () => ({
  StoreForm: () => <div role="form">新建店铺表单</div>,
}));

import NewWorkbenchStorePage from "@/app/workbench/stores/new/page";

describe("NewWorkbenchStorePage", () => {
  afterEach(() => {
    context.isLoading = false;
    context.roles = ["listingkit_operator"];
    context.effectiveOrganization = { id: "org-a", name: "企业 A", roles: [] };
  });

  it("does not mount the create form for an organization without create role", () => {
    context.roles = ["listingkit_viewer"];
    render(<NewWorkbenchStorePage />);
    expect(screen.getByRole("alert")).toHaveTextContent("没有创建当前企业店铺的权限");
    expect(screen.queryByRole("form")).not.toBeInTheDocument();
  });

  it("waits for context before deciding whether the create form is allowed", () => {
    context.isLoading = true;
    context.roles = [];
    render(<NewWorkbenchStorePage />);
    expect(screen.getByRole("status")).toHaveTextContent("正在加载企业权限");
  });
});
