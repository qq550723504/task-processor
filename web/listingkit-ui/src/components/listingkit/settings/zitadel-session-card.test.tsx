import { render, screen, waitFor } from "@testing-library/react";

import { ZitadelSessionCard } from "@/components/listingkit/settings/zitadel-session-card";

describe("ZitadelSessionCard", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("shows tenant, user and platform admin status from the session endpoint", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        ok: true,
        identity: {
          tenantId: "org-286",
          userId: "user-42",
          userType: "zitadel",
          roles: ["platform_admin", "billing_admin"],
        },
      }),
    } as Response);

    render(<ZitadelSessionCard />);

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
          roles: ["tenant_user"],
        },
      }),
    } as Response);

    render(<ZitadelSessionCard />);

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

    render(<ZitadelSessionCard />);

    await waitFor(() => {
      expect(screen.getByText("Missing ZITADEL bearer token")).toBeInTheDocument();
    });
  });
});
