import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  useZitadelIdentity,
  ZitadelAuthGate,
} from "@/components/providers/zitadel-auth-gate";

function IdentityProbe() {
  const identity = useZitadelIdentity();

  return <div>{identity?.roles?.join(",") ?? "no identity"}</div>;
}

describe("ZitadelAuthGate", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("bypasses session verification on the login page", () => {
    vi.spyOn(window, "fetch");
    window.history.replaceState({}, "", "/login?returnTo=%2F");

    render(
      <ZitadelAuthGate>
        <div>login page</div>
      </ZitadelAuthGate>,
    );

    expect(screen.getByText("login page")).toBeInTheDocument();
    expect(window.fetch).not.toHaveBeenCalled();

    window.history.replaceState({}, "", "/");
  });

  it("provides verified session identity to descendants", async () => {
    vi.spyOn(window, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          ok: true,
          identity: { roles: ["listingkit_admin"] },
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    render(
      <ZitadelAuthGate>
        <IdentityProbe />
      </ZitadelAuthGate>,
    );

    expect(screen.getByText("正在验证登录状态...")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("listingkit_admin")).toBeInTheDocument();
    });
  });
});
