import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactElement } from "react";

import { PlatformSubscriptionPage } from "@/components/listingkit/subscription/platform-subscription-page";
import {
  applyPlatformTenantSubscriptionPlan,
  getPlatformSubscriptionPlans,
  getPlatformTenantSubscriptionAuditLogs,
  getPlatformTenantDirectory,
  getPlatformTenantSubscriptions,
  getPlatformTenantSubscription,
  invitePlatformTenantMember,
  updatePlatformTenantSubscriptionUsage,
  updatePlatformTenantSubscriptionEntitlement,
} from "@/lib/api/subscription";
import { ApiError } from "@/lib/api/client";

vi.mock("@/lib/api/subscription", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/lib/api/subscription")>();
  return {
    ...actual,
    applyPlatformTenantSubscriptionPlan: vi.fn(),
    getPlatformSubscriptionPlans: vi.fn(),
    getPlatformTenantSubscriptionAuditLogs: vi.fn(),
    getPlatformTenantDirectory: vi.fn(),
    getPlatformTenantSubscriptions: vi.fn(),
    getPlatformTenantSubscription: vi.fn(),
    invitePlatformTenantMember: vi.fn(),
    updatePlatformTenantSubscriptionUsage: vi.fn(),
    updatePlatformTenantSubscriptionEntitlement: vi.fn(),
  };
});

const mockedApplyPlatformTenantSubscriptionPlan = vi.mocked(
  applyPlatformTenantSubscriptionPlan,
);
const mockedGetPlatformSubscriptionPlans = vi.mocked(getPlatformSubscriptionPlans);
const mockedGetPlatformTenantSubscriptionAuditLogs = vi.mocked(
  getPlatformTenantSubscriptionAuditLogs,
);
const mockedGetPlatformTenantDirectory = vi.mocked(getPlatformTenantDirectory);
const mockedGetPlatformTenantSubscriptions = vi.mocked(
  getPlatformTenantSubscriptions,
);
const mockedGetPlatformTenantSubscription = vi.mocked(getPlatformTenantSubscription);
const mockedInvitePlatformTenantMember = vi.mocked(invitePlatformTenantMember);
const mockedUpdatePlatformTenantSubscriptionUsage = vi.mocked(
  updatePlatformTenantSubscriptionUsage,
);
const mockedUpdatePlatformTenantSubscriptionEntitlement = vi.mocked(
  updatePlatformTenantSubscriptionEntitlement,
);

describe("PlatformSubscriptionPage", () => {
  beforeEach(() => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    mockedGetPlatformTenantSubscriptionAuditLogs.mockReset();
    mockedGetPlatformTenantDirectory.mockReset();
    mockedApplyPlatformTenantSubscriptionPlan.mockReset();
    mockedGetPlatformSubscriptionPlans.mockReset();
    mockedGetPlatformTenantSubscriptions.mockReset();
    mockedGetPlatformTenantSubscription.mockReset();
    mockedInvitePlatformTenantMember.mockReset();
    mockedUpdatePlatformTenantSubscriptionUsage.mockReset();
    mockedUpdatePlatformTenantSubscriptionEntitlement.mockReset();
    mockedInvitePlatformTenantMember.mockResolvedValue({
      tenant_id: "org-target",
      user_id: "user-1",
      email: "jane@example.com",
      role: "listingkit_viewer",
      authorization_id: "authorization-1",
      invitation_email_sent: true,
    });
    mockedGetPlatformSubscriptionPlans.mockResolvedValue([
      {
        plan: {
          code: "professional",
          name: "专业版",
          sort_order: 20,
          active: true,
        },
        modules: [
          {
            plan_code: "professional",
            module_code: "studio",
            limits: { design_jobs: 100 },
            sort_order: 50,
          },
        ],
      },
    ]);
    mockedGetPlatformTenantSubscriptionAuditLogs.mockResolvedValue([]);
    mockedGetPlatformTenantSubscriptions.mockResolvedValue([
      {
        tenant_id: "org-target",
        tenant_display_name: "目标租户",
        entitlement_count: 1,
        active_count: 0,
      },
    ]);
    mockedGetPlatformTenantDirectory.mockResolvedValue([
      {
        tenant_id: "org-target",
        tenant_display_name: "目标租户",
        primary_domain: "target.example",
        state: "ORGANIZATION_STATE_ACTIVE",
      },
      {
        tenant_id: "org-new",
        tenant_display_name: "新租户",
        primary_domain: "new.example",
        state: "ORGANIZATION_STATE_ACTIVE",
      },
    ]);
  });

  it("loads a tenant subscription and updates a module entitlement", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            sort_order: 50,
            active: true,
          },
          usage: [],
          allowed: false,
          reason: "not_configured",
          used: {},
          limits: {},
        },
      ],
    });
    mockedUpdatePlatformTenantSubscriptionEntitlement.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      module_code: "studio",
      status: "active",
      limits: { design_jobs: 10 },
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    expect(await screen.findByText("Studio")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByRole("button", { name: "添加 设计任务额度" }));
    fireEvent.change(screen.getByLabelText("额度值 design_jobs"), {
      target: { value: "10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionEntitlement).toHaveBeenCalledWith(
        "org-target",
        "studio",
        expect.objectContaining({
          status: "active",
          limits: { design_jobs: 10 },
        }),
      );
    });
  });

  it("applies a subscription plan to a tenant", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedApplyPlatformTenantSubscriptionPlan.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      plan_code: "professional",
      status: "active",
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("专业版");
    fireEvent.change(screen.getByLabelText("套餐"), {
      target: { value: "professional" },
    });
    fireEvent.click(screen.getByRole("button", { name: "应用套餐" }));

    await waitFor(() => {
      expect(mockedApplyPlatformTenantSubscriptionPlan).toHaveBeenCalledWith(
        "org-target",
        expect.objectContaining({
          plan_code: "professional",
          status: "active",
        }),
      );
    });
  });

  it("opens a new tenant directly from subscription management", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-new",
      modules: [],
      entitlements: [],
    });
    mockedApplyPlatformTenantSubscriptionPlan.mockResolvedValue({
      id: 2,
      tenant_id: "org-new",
      plan_code: "professional",
      status: "active",
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.click(screen.getByRole("button", { name: "开通新租户" }));
    await waitFor(() => {
      expect(screen.getByLabelText("新租户套餐").querySelectorAll("option")).toHaveLength(2);
    });
    fireEvent.change(screen.getByLabelText("新租户 ID"), {
      target: { value: "org-new" },
    });
    fireEvent.change(screen.getByLabelText("新租户套餐"), {
      target: { value: "professional" },
    });
    fireEvent.click(screen.getByRole("button", { name: "确认开通" }));

    await waitFor(() => {
      expect(mockedApplyPlatformTenantSubscriptionPlan).toHaveBeenCalledWith(
        "org-new",
        expect.objectContaining({
          plan_code: "professional",
          status: "active",
        }),
      );
    });
  });

  it("applies OSS storage limit presets as byte limits", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "oss_storage",
            name: "OSS 存储",
            sort_order: 60,
            active: true,
          },
          usage: [],
          allowed: false,
          reason: "not_configured",
          used: {},
          limits: {},
        },
      ],
    });
    mockedUpdatePlatformTenantSubscriptionEntitlement.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      module_code: "oss_storage",
      status: "active",
      limits: { storage_bytes: 1073741824 },
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("OSS 存储");
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByRole("button", { name: "1 GB" }));
    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionEntitlement).toHaveBeenCalledWith(
        "org-target",
        "oss_storage",
        expect.objectContaining({
          limits: { storage_bytes: 1073741824 },
        }),
      );
    });
  });

  it("loads a tenant from the configured tenant list", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.click(await screen.findByText("目标租户"));

    await waitFor(() => {
      expect(mockedGetPlatformTenantSubscription).toHaveBeenCalledWith(
        "org-target",
      );
    });
  });

  it("filters tenants by display name and still keeps tenant id as fallback", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "目标" },
    });

    expect(await screen.findByText("目标租户")).toBeInTheDocument();
    expect(screen.getByText("org-target")).toBeInTheDocument();
  });

  it("adjusts module usage for a billing period", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "oss_storage",
            name: "OSS 存储",
            sort_order: 60,
            active: true,
          },
          entitlement: {
            id: 1,
            tenant_id: "org-target",
            module_code: "oss_storage",
            status: "active",
            limits: { storage_bytes: 1048576 },
          },
          usage: [
            {
              id: 1,
              tenant_id: "org-target",
              module_code: "oss_storage",
              period_key: "2026-05",
              metric: "storage_bytes",
              used: 2048,
            },
          ],
          allowed: true,
          used: { storage_bytes: 2048 },
          limits: { storage_bytes: 1048576 },
        },
      ],
    });
    mockedUpdatePlatformTenantSubscriptionUsage.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      module_code: "oss_storage",
      period_key: "2026-05",
      metric: "storage_bytes",
      used: 0,
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("OSS 存储");
    expect(screen.getByText("存储额度: 1 MB")).toBeInTheDocument();
    expect(screen.getByText("存储额度: 2 KB")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByText("高级操作：用量调整"));
    fireEvent.click(screen.getByRole("button", { name: "重置为 0" }));
    fireEvent.click(screen.getByRole("button", { name: "保存用量" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionUsage).toHaveBeenCalledWith(
        "org-target",
        "oss_storage",
        expect.objectContaining({
          metric: "storage_bytes",
          period_key: "2026-05",
          used: 0,
        }),
      );
    });
  });

  it("shows operator guidance and module business summary", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            description: "Design jobs",
            sort_order: 50,
            active: true,
          },
          usage: [],
          allowed: false,
          reason: "not_configured",
          used: {},
          limits: {},
        },
      ],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    expect(
      screen.getByText("可按名称、域名或租户 ID 搜索整个 ZITADEL 租户目录；开通套餐前无需先在订阅表中出现。"),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    expect(await screen.findByText("控制生成任务、工作台和图片生产类能力。")).toBeInTheDocument();
  });

  it("shows usage adjustment as an advanced action", () => {
    renderWithQueryClient(<PlatformSubscriptionPage />);

    expect(screen.getByText("高级操作：用量调整")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存用量" })).toBeDisabled();
  });

  it("uses mobile-first tenant search and tenant list layouts", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    await screen.findByText("目标租户");

    expect(screen.getByPlaceholderText("搜索或输入租户 ID")).not.toHaveClass("min-w-[260px]");
    expect(screen.getByRole("button", { name: "查询" })).toHaveClass("w-full");
    expect(screen.getAllByRole("button", { name: "刷新" })[0]).toHaveClass("w-full");
  });

  it("submits a viewer invitation only after a tenant is selected", async () => {
    const user = userEvent.setup();
    const confirmMock = vi.mocked(globalThis.confirm);
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    expect(screen.queryByRole("button", { name: "邀请成员" })).not.toBeInTheDocument();
    await user.click(await screen.findByText("目标租户"));
    await user.click(await screen.findByRole("button", { name: "邀请成员" }));

    const roleSelect = screen.getByLabelText("角色");
    expect(roleSelect).toHaveValue("listingkit_viewer");
    expect(roleSelect.querySelector('option[value="platform_admin"]')).toBeNull();

    await user.type(screen.getByLabelText("名字"), "Jane");
    await user.type(screen.getByLabelText("姓氏"), "Doe");
    await user.type(screen.getByLabelText("邮箱"), "jane@example.com");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    await waitFor(() => {
      expect(confirmMock).toHaveBeenCalledWith(
        expect.stringMatching(/org-target.*j\*\*\*@example\.com.*listingkit_viewer/),
      );
      expect(mockedInvitePlatformTenantMember).toHaveBeenCalledWith(
        "org-target",
        {
          given_name: "Jane",
          family_name: "Doe",
          email: "jane@example.com",
          role: "listingkit_viewer",
        },
      );
    });
  });

  it("submits an email invitation with E.164 phone verification and a username", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await user.selectOptions(screen.getByLabelText("邀请方式"), "email_phone");
    await fillValidInvitation(user);
    await user.type(screen.getByLabelText("手机号"), "+8613812345678");
    await user.type(screen.getByLabelText("用户名"), "jane.doe");
    expect(screen.getByLabelText("手机号")).toHaveValue("+8613812345678");
    expect(screen.getByLabelText("手机号")).toBeValid();
    expect(screen.getByLabelText("用户名")).toHaveValue("jane.doe");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    await waitFor(() => {
      expect(mockedInvitePlatformTenantMember).toHaveBeenCalledWith(
        "org-target",
        {
          given_name: "Jane",
          family_name: "Doe",
          email: "jane@example.com",
          phone: "+8613812345678",
          username: "jane.doe",
          role: "listingkit_viewer",
        },
      );
    });
  });

  it("blocks a phone-verification invitation without an email", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await user.selectOptions(screen.getByLabelText("邀请方式"), "email_phone");
    await user.type(screen.getByLabelText("名字"), "Jane");
    await user.type(screen.getByLabelText("姓氏"), "Doe");
    await user.type(screen.getByLabelText("手机号"), "+8613812345678");
    await user.type(screen.getByLabelText("用户名"), "jane.doe");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    expect(mockedInvitePlatformTenantMember).not.toHaveBeenCalled();
  });

  it("rejects a phone-verification invitation with a non-E.164 phone", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await user.selectOptions(screen.getByLabelText("邀请方式"), "email_phone");
    await fillValidInvitation(user);
    await user.type(screen.getByLabelText("手机号"), "13812345678");
    await user.type(screen.getByLabelText("用户名"), "jane.doe");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    expect(mockedInvitePlatformTenantMember).not.toHaveBeenCalled();
  });

  it("omits stale phone verification fields after switching back to email-only", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await user.selectOptions(screen.getByLabelText("邀请方式"), "email_phone");
    await fillValidInvitation(user);
    await user.type(screen.getByLabelText("手机号"), "+8613812345678");
    await user.type(screen.getByLabelText("用户名"), "jane.doe");
    await user.selectOptions(screen.getByLabelText("邀请方式"), "email_only");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    await waitFor(() => {
      expect(mockedInvitePlatformTenantMember).toHaveBeenCalledWith(
        "org-target",
        {
          given_name: "Jane",
          family_name: "Doe",
          email: "jane@example.com",
          role: "listingkit_viewer",
        },
      );
    });
  });

  it("clears and closes the form after ZITADEL sends initialization mail", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await fillValidInvitation(user);
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    expect(
      await screen.findByText(
        /org-target.*j\*\*\*@example\.com.*listingkit_viewer.*初始化邮件/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("邮箱")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "邀请成员" }));
    expect(screen.getByLabelText("名字")).toHaveValue("");
    expect(screen.getByLabelText("姓氏")).toHaveValue("");
    expect(screen.getByLabelText("邮箱")).toHaveValue("");
    expect(screen.getByLabelText("角色")).toHaveValue("listingkit_viewer");
  });

  it("shows incomplete access guidance when the API returns an incomplete invitation", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedInvitePlatformTenantMember.mockRejectedValue(
      new ApiError("ListingKit API request failed: 502", 502, {
        error: "zitadel_member_invitation_incomplete",
        user_id: "user-1",
      }),
    );

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await fillValidInvitation(user);
    await user.selectOptions(
      screen.getByLabelText("角色"),
      "listingkit_operator",
    );
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    const guidance = await screen.findByText(/u\*\*\*1/);
    expect(guidance).toHaveTextContent("尚未获得访问权限");
    expect(guidance).toHaveTextContent("listingkit_operator");
  });

  it("uses native validation to block missing or invalid invitation fields", async () => {
    const user = userEvent.setup();
    const confirmMock = vi.mocked(globalThis.confirm);
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);

    await user.click(screen.getByRole("button", { name: "发送邀请" }));
    expect(mockedInvitePlatformTenantMember).not.toHaveBeenCalled();
    expect(confirmMock).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("名字"), "Jane");
    await user.type(screen.getByLabelText("姓氏"), "Doe");
    await user.type(screen.getByLabelText("邮箱"), "not-an-email");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    expect(mockedInvitePlatformTenantMember).not.toHaveBeenCalled();
    expect(confirmMock).not.toHaveBeenCalled();
  });

  it("locks the invitation and tenant context while the request is pending", async () => {
    const user = userEvent.setup();
    const invitationRequest = createDeferred<
      Awaited<ReturnType<typeof invitePlatformTenantMember>>
    >();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedInvitePlatformTenantMember.mockReturnValue(invitationRequest.promise);

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await fillValidInvitation(user);
    await user.selectOptions(screen.getByLabelText("邀请方式"), "email_phone");
    await user.type(screen.getByLabelText("手机号"), "+8613812345678");
    await user.type(screen.getByLabelText("用户名"), "jane.doe");
    await user.selectOptions(
      screen.getByLabelText("角色"),
      "listingkit_operator",
    );
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    await waitFor(() => {
      expect(mockedInvitePlatformTenantMember).toHaveBeenCalledOnce();
    });
    expect(screen.getByLabelText("租户 ID")).toBeDisabled();
    expect(screen.getByRole("button", { name: "查询" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "开通新租户" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "取消" })).toBeDisabled();
    expect(screen.getByLabelText("名字")).toBeDisabled();
    expect(screen.getByLabelText("姓氏")).toBeDisabled();
    expect(screen.getByLabelText("邮箱")).toBeDisabled();
    expect(screen.getByLabelText("邀请方式")).toBeDisabled();
    expect(screen.getByLabelText("手机号")).toBeDisabled();
    expect(screen.getByLabelText("用户名")).toBeDisabled();
    expect(screen.getByLabelText("角色")).toBeDisabled();
    expect(screen.getByRole("button", { name: "发送邀请" })).toBeDisabled();
    expect(screen.getByText("目标租户").closest("button")).toBeDisabled();

    await act(async () => {
      invitationRequest.resolve({
        tenant_id: "org-target",
        user_id: "user-2",
        email: "jane@example.com",
        role: "listingkit_operator",
        authorization_id: "authorization-2",
        invitation_email_sent: true,
      });
      await invitationRequest.promise;
    });

    expect(
      await screen.findByText(
        /org-target.*j\*\*\*@example\.com.*listingkit_operator.*初始化邮件/,
      ),
    ).toBeInTheDocument();
  });

  it("blocks invitations while tenant onboarding is pending", async () => {
    const user = userEvent.setup();
    const openingRequest = createDeferred<
      Awaited<ReturnType<typeof applyPlatformTenantSubscriptionPlan>>
    >();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedApplyPlatformTenantSubscriptionPlan.mockReturnValue(
      openingRequest.promise,
    );

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await fillValidInvitation(user);
    await user.click(screen.getByRole("button", { name: "开通新租户" }));
    await user.type(screen.getByLabelText("新租户 ID"), "org-new");
    await user.selectOptions(screen.getByLabelText("新租户套餐"), "professional");
    await user.click(screen.getByRole("button", { name: "确认开通" }));

    await waitFor(() => {
      expect(mockedApplyPlatformTenantSubscriptionPlan).toHaveBeenCalledOnce();
    });
    const invitationForm = screen
      .getByRole("button", { name: "发送邀请" })
      .closest("form");
    expect(invitationForm).not.toBeNull();
    expect(screen.getByRole("button", { name: "发送邀请" })).toBeDisabled();
    fireEvent.submit(invitationForm!);
    expect(mockedInvitePlatformTenantMember).not.toHaveBeenCalled();
    expect(screen.getByLabelText("租户 ID")).toHaveValue("org-target");

    await act(async () => {
      openingRequest.resolve({
        id: 2,
        tenant_id: "org-new",
        plan_code: "professional",
        status: "active",
      });
      await openingRequest.promise;
    });
    await waitFor(() => {
      expect(screen.getByLabelText("租户 ID")).toHaveValue("org-new");
    });
  });

  it("blocks onboarding while an invitation refreshes its frozen tenant audit", async () => {
    const user = userEvent.setup();
    const invitationRequest = createDeferred<
      Awaited<ReturnType<typeof invitePlatformTenantMember>>
    >();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedInvitePlatformTenantMember.mockReturnValue(invitationRequest.promise);

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await fillValidInvitation(user);
    await user.click(screen.getByRole("button", { name: "开通新租户" }));
    await user.type(screen.getByLabelText("新租户 ID"), "org-new");
    await user.selectOptions(screen.getByLabelText("新租户套餐"), "professional");
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    await waitFor(() => {
      expect(mockedInvitePlatformTenantMember).toHaveBeenCalledOnce();
    });
    const onboardingForm = screen
      .getByRole("button", { name: "确认开通" })
      .closest("form");
    expect(onboardingForm).not.toBeNull();
    fireEvent.submit(onboardingForm!);
    expect(mockedApplyPlatformTenantSubscriptionPlan).not.toHaveBeenCalled();

    await act(async () => {
      invitationRequest.resolve({
        tenant_id: "org-target",
        user_id: "user-3",
        email: "jane@example.com",
        role: "listingkit_viewer",
        authorization_id: "authorization-3",
        invitation_email_sent: true,
      });
      await invitationRequest.promise;
    });

    await screen.findByText(
      /org-target.*j\*\*\*@example\.com.*listingkit_viewer.*初始化邮件/,
    );
    expect(screen.getByLabelText("租户 ID")).toHaveValue("org-target");
    await waitFor(() => {
      expect(mockedGetPlatformTenantSubscriptionAuditLogs.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
    expect(
      mockedGetPlatformTenantSubscriptionAuditLogs.mock.calls.every(
        ([tenantId]) => tenantId === "org-target",
      ),
    ).toBe(true);
  });

  it("formats remaining invitation failures with the subscription API formatter", async () => {
    const user = userEvent.setup();
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedInvitePlatformTenantMember.mockRejectedValue(
      new ApiError("ListingKit API request failed: 409", 409, {
        error: "member_invitation_conflict",
      }),
    );

    renderWithQueryClient(<PlatformSubscriptionPage />);
    await selectTenantAndOpenInvitation(user);
    await fillValidInvitation(user);
    await user.click(screen.getByRole("button", { name: "发送邀请" }));

    expect(
      await screen.findByText("ListingKit API request failed: 409"),
    ).toBeInTheDocument();
  });
});

async function selectTenantAndOpenInvitation(
  user: ReturnType<typeof userEvent.setup>,
) {
  await user.click(await screen.findByText("目标租户"));
  await user.click(await screen.findByRole("button", { name: "邀请成员" }));
}

async function fillValidInvitation(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("名字"), "Jane");
  await user.type(screen.getByLabelText("姓氏"), "Doe");
  await user.type(screen.getByLabelText("邮箱"), "jane@example.com");
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function renderWithQueryClient(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}
