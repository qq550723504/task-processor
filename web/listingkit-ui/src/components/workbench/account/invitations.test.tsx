import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { RecipientInvitation } from "./invitations";

const id = "4841d296-ef14-4c16-8d25-a7667e534feb";
const invitation = {
  id,
  organizationId: "target",
  organizationName: "受邀企业",
  creatorId: "admin",
  contact: "recipient@example.test",
  role: "sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_01",
  permissions: [],
  state: "pending",
  revision: 1,
  createdAt: "2026-09-28T00:00:00Z",
  expiresAt: "2026-10-05T00:00:00Z",
  recipientId: "",
  authorizationId: "",
  deliveryState: "mail_server_accepted",
  deliveryAttempts: 1,
  deliveryUpdatedAt: "2026-09-28T00:00:00Z",
};
const clients: QueryClient[] = [];
afterEach(() => {
  cleanup();
  clients.splice(0).forEach((c) => c.clear());
  vi.unstubAllGlobals();
});
function page() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  clients.push(client);
  return (
    <StrictMode>
      <QueryClientProvider client={client}>
        <RecipientInvitation userId="recipient" id={id} />
      </QueryClientProvider>
    </StrictMode>
  );
}
function response(change: Record<string, unknown> = {}) {
  return Response.json({
    schemaVersion: "membership-invitation-v1",
    userId: "recipient",
    organizationId: "target",
    invitation: { ...invitation, ...change },
  });
}
it("requires explicit consent without any selected enterprise and only queries an unknown grant", async () => {
  let accepting = false;
  const calls = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === "POST") accepting = true;
    return Promise.resolve(
      response(
        accepting
          ? { state: "accepting", revision: 2, recipientId: "recipient" }
          : {},
      ),
    );
  });
  vi.stubGlobal("fetch", calls);
  render(page());
  const accept = await screen.findByRole("button", { name: "接受邀请" });
  expect(screen.getByText("受邀企业")).toBeVisible();
  expect(calls.mock.calls.every(([, init]) => init?.method === "GET")).toBe(
    true,
  );
  expect(
    calls.mock.calls.every(
      ([, init]) =>
        !new Headers(init?.headers).has("X-Expected-Organization-ID"),
    ),
  ).toBe(true);
  const user = userEvent.setup();
  await user.dblClick(accept);
  await screen.findByText("加入结果待核实", { exact: true });
  expect(
    screen.queryByRole("button", { name: "接受邀请" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "查询原邀请" }));
  await waitFor(() =>
    expect(
      calls.mock.calls.filter(([, init]) => init?.method === "GET").length,
    ).toBeGreaterThan(1),
  );
  expect(
    calls.mock.calls.filter(([, init]) => init?.method === "POST"),
  ).toHaveLength(1);
});
it("keeps invitation details hidden when the signed-in email is not verified or does not match", async () => {
  const calls = vi.fn(() =>
    Promise.resolve(
      Response.json({ code: "PERMISSION_DENIED" }, { status: 403 }),
    ),
  );
  vi.stubGlobal("fetch", calls);
  render(page());
  const login = await screen.findByRole("link", { name: "前往官方登录" });
  expect(login).toHaveAttribute(
    "href",
    `/api/zitadel-auth/login?returnTo=${encodeURIComponent(`/invitations/${id}`)}`,
  );
  expect(screen.queryByText("受邀企业")).not.toBeInTheDocument();
  expect(screen.queryByText(invitation.contact)).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "接受邀请" }),
  ).not.toBeInTheDocument();
  expect(calls.mock.calls.length).toBeGreaterThan(0);
});
it("aborts a consent request when leaving the invitation page", async () => {
  let signal: AbortSignal | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method !== "POST") return Promise.resolve(response());
      signal = init.signal ?? undefined;
      return new Promise<Response>((_, reject) =>
        signal?.addEventListener(
          "abort",
          () => reject(new DOMException("aborted", "AbortError")),
          { once: true },
        ),
      );
    }),
  );
  const view = render(page());
  await userEvent
    .setup()
    .click(await screen.findByRole("button", { name: "拒绝邀请" }));
  expect(signal?.aborted).toBe(false);
  view.unmount();
  expect(signal?.aborted).toBe(true);
});
