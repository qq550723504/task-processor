import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, cleanup, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { afterEach, it, expect, vi } from "vitest";
import { IdentityDates, RegionSettings } from "./account-facts";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it.each(["self", "another-user"])("shows only the current subject's official session authentication time: %s", async userId => {
  vi.stubGlobal("fetch", vi.fn((path: string) => Promise.resolve(Response.json(path === "/api/zitadel-auth/session"
    ? { ok: true, identity: { userId }, authenticatedAt: "2026-09-28T01:00:00.000Z" }
    : { schemaVersion: "account-identity-facts-v1", userId: "self", registeredAt: "2026-09-01T00:00:00Z", lastLogin: null, passwordChangedAt: null, source: "zitadel_auth_v1", readAt: "2026-09-28T02:00:00Z" }))));
  const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><IdentityDates subject="self" /></QueryClientProvider>);
  await screen.findByText(/当前会话认证时间/);
  await waitFor(() => expect(container.querySelectorAll('time[datetime="2026-09-28T01:00:00.000Z"]')).toHaveLength(userId === "self" ? 1 : 0));
  expect(screen.queryByText(/暂无登录记录/)).not.toBeInTheDocument();
  if (userId !== "self") expect(await screen.findByText(/认证时间暂不可用/)).toBeVisible();
});
it("saves actual region preferences after StrictMode effect replay", async () => {
  const initial = {
    schemaVersion: "account-preferences-v1",
    userId: "self",
    country: "",
    province: "",
    city: "",
    updatedAt: null,
    readAt: "2026-09-28T00:00:00Z",
    source: "account_profile",
  };
  const calls = vi.fn((_: string, init: RequestInit) =>
    Promise.resolve(
      Response.json(
        init.method === "PUT"
          ? {
              ...initial,
              ...JSON.parse(String(init.body)),
              updatedAt: "2026-09-28T01:00:00Z",
            }
          : initial,
      ),
    ),
  );
  vi.stubGlobal("fetch", calls);
  render(
    <StrictMode>
      <QueryClientProvider client={new QueryClient()}>
        <RegionSettings subject="self" />
      </QueryClientProvider>
    </StrictMode>,
  );
  const user = userEvent.setup();
  await user.type(await screen.findByRole("textbox", { name: "城市" }), "杭州");
  await user.click(screen.getByRole("button", { name: "保存地区资料" }));
  await waitFor(() =>
    expect(calls.mock.calls.some(([, init]) => init.method === "PUT")).toBe(
      true,
    ),
  );
  expect(screen.getByRole("textbox", { name: "城市" })).toHaveValue("杭州");
});
