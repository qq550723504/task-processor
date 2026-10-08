import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";

import { ZitadelSessionCard } from "@/components/listingkit/settings/zitadel-session-card";

describe("ZitadelSessionCard", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("shows account identity and platform admin status from the session endpoint", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        ok: true,
        identity: {
          tenantId: "org-286",
          userId: "user-42",
          username: "admin",
          userType: "zitadel",
          roles: ["platform_admin", "billing_admin"], permissions: ["listingkit.admin.read","listingkit.admin.write","product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.agent.configure","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.knowledge.manage","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.store.delete","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.organization_member.manage","workbench.commercial.read","workbench.commercial.purchase","workbench.commercial.wallet_topup"],
        },
      }),
    } as Response);

    renderWithQueryClient(<ZitadelSessionCard />);

    expect(await screen.findByText("admin")).toBeInTheDocument();
    expect(screen.getByText("登录名")).toBeInTheDocument();
    expect(await screen.findByText("org-286")).toBeInTheDocument();
    expect(screen.getByText("user-42")).toBeInTheDocument();
    expect(screen.getByText("platform_admin")).toBeInTheDocument();
    expect(screen.getByText("具备平台管理权限")).toBeInTheDocument();
  });

  it("shows missing platform admin role guidance", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        ok: true,
        identity: {
          tenantId: "org-286",
          userId: "user-42",
          roles: ["tenant_user"], permissions: [],
        },
      }),
    } as Response);

    renderWithQueryClient(<ZitadelSessionCard />);

    expect(await screen.findByText("缺少平台管理权限")).toBeInTheDocument();
    expect(screen.getByText(/platform_admin/)).toBeInTheDocument();
  });

  it("shows session endpoint errors", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({
        error: "zitadel_token_invalid",
        message: "Missing ZITADEL bearer token",
      }),
    } as Response);

    renderWithQueryClient(<ZitadelSessionCard />);

    await waitFor(() => {
      expect(screen.getByText("Missing ZITADEL bearer token")).toBeInTheDocument();
    });
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}
