import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, cleanup, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { afterEach, it, expect, vi } from "vitest";
import { RegionSettings } from "./account-facts";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
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
