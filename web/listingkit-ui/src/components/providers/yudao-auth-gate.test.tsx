import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { YudaoAuthGate } from "@/components/providers/yudao-auth-gate";
import { clearYudaoAuth, rememberYudaoAuth } from "@/lib/api/yudao-auth";

describe("YudaoAuthGate", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    window.sessionStorage.clear();
  });

  it("renders the workbench while waiting for yudao auth", async () => {
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <YudaoAuthGate authWaitMs={0}>
        <div>ListingKit workbench</div>
      </YudaoAuthGate>,
    );

    await waitFor(() => {
      expect(screen.getByText("ListingKit workbench")).toBeInTheDocument();
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("verifies stored yudao auth in the background", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    rememberYudaoAuth({
      accessToken: "access-token-1",
      tenantId: "1",
      visitTenantId: "2",
    });

    render(
      <YudaoAuthGate authWaitMs={0}>
        <div>ListingKit workbench</div>
      </YudaoAuthGate>,
    );

    await waitFor(() => {
      expect(screen.getByText("ListingKit workbench")).toBeInTheDocument();
    });
    const headers = new Headers(fetchMock.mock.calls[0]?.[1]?.headers);
    expect(headers.get("Authorization")).toBe("Bearer access-token-1");
    expect(headers.get("tenant-id")).toBe("1");
    expect(headers.get("visit-tenant-id")).toBe("2");
  });

  it("clears stale auth without blocking the workbench", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "yudao_token_invalid" }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
      ),
    );
    rememberYudaoAuth({
      accessToken: "expired-token",
      tenantId: "1",
    });

    render(
      <YudaoAuthGate authWaitMs={0}>
        <div>ListingKit workbench</div>
      </YudaoAuthGate>,
    );

    await waitFor(() => {
      expect(screen.getByText("ListingKit workbench")).toBeInTheDocument();
    });
    expect(window.sessionStorage.getItem("listingkit:yudao-auth")).toBeNull();
    clearYudaoAuth();
  });
});
