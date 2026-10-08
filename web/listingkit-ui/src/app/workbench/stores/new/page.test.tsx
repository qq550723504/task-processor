import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const context = vi.hoisted(() => ({
  isLoading: false,
  effectiveOrganization: { id: "org-a", name: "企业 A", roles: [], permissions: [] as string[] },
  roles: ["listingkit_operator"], permissions: ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"],
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
    context.roles = ["listingkit_operator"]; context.permissions = ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"];
    context.effectiveOrganization = { id: "org-a", name: "企业 A", roles: [], permissions: [] };
  });

  it.each(["listingkit_admin", "sumi_role_0123456789abcdef0123456789abcdef_01"])("mounts the form for projected create permission with native role %s", (role) => {
    context.roles = [role];
    context.permissions = ["workbench.store.create"];
    render(<NewWorkbenchStorePage />);
    expect(screen.getByRole("form")).toHaveTextContent("新建店铺表单");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not mount the create form for an organization without create role", () => {
    context.roles = ["listingkit_viewer"]; context.permissions = ["workbench.task.read","workbench.store.read","workbench.source_account.read","workbench.organization_member.read","workbench.commercial.read"];
    render(<NewWorkbenchStorePage />);
    expect(screen.getByRole("alert")).toHaveTextContent("没有创建当前企业店铺的权限");
    expect(screen.queryByRole("form")).not.toBeInTheDocument();
  });

  it("waits for context before deciding whether the create form is allowed", () => {
    context.isLoading = true;
    context.roles = []; context.permissions = [];
    render(<NewWorkbenchStorePage />);
    expect(screen.getByRole("status")).toHaveTextContent("正在加载企业权限");
  });
});
