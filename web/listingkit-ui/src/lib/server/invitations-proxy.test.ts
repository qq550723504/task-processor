import { afterEach, it, expect, vi } from "vitest";
import { proxyRecipientInvitation } from "./invitations-proxy";
import { proxyMembers } from "./members-proxy";
const id = "4841d296-ef14-4c16-8d25-a7667e534feb";
const item = {
  id,
  organizationId: "target",
  organizationName: "Target",
  creatorId: "admin",
  contact: "recipient@example.test",
  role: "listingkit_viewer",
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
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
it("reads recipient invitation without any selected enterprise and rejects a different subject", async () => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  const fetch = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) =>
    Promise.resolve(
      Response.json({
        schemaVersion: "membership-invitation-v1",
        userId: "recipient",
        organizationId: "target",
        invitation: item,
      }),
    ),
  );
  vi.stubGlobal("fetch", fetch);
  const request = () =>
    new Request(`http://localhost:3000/api/account/invitations/${id}`, {
      headers: { "X-Expected-User-ID": "recipient" },
    });
  expect(
    (await proxyRecipientInvitation(request(), "private-token", "recipient"))
      .status,
  ).toBe(200);
  expect(fetch.mock.calls[0][0]).toContain(`/api/v1/account/invitations/${id}`);
  expect(
    new Headers(fetch.mock.calls[0][1]?.headers).has(
      "X-Requested-Organization-ID",
    ),
  ).toBe(false);
  fetch.mockImplementationOnce(() =>
    Promise.resolve(
      Response.json({
        schemaVersion: "membership-invitation-v1",
        userId: "foreign",
        organizationId: "target",
        invitation: item,
      }),
    ),
  );
  expect(
    (await proxyRecipientInvitation(request(), "private-token", "recipient"))
      .status,
  ).toBe(409);
});
it("rejects cross-site actions and input payload before dispatch", async () => {
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000");
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  for (const fields of [
    { origin: "http://foreign.test", body: undefined, status: 403 },
    {
      origin: "http://localhost:3000",
      body: '{"organizationId":"foreign"}',
      status: 400,
    },
  ]) {
    const request = new Request(
      `http://localhost:3000/api/account/invitations/${id}/accept`,
      {
        method: "POST",
        headers: { Origin: fields.origin, "X-Expected-User-ID": "recipient" },
        body: fields.body,
      },
    );
    expect(
      (await proxyRecipientInvitation(request, "private-token", "recipient"))
        .status,
    ).toBe(fields.status);
  }
  expect(fetch).not.toHaveBeenCalled();
});
it("closes immediate member creation and binds full-directory summaries", async () => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000");
  const headers = {
    Origin: "http://localhost:3000",
    "X-Expected-User-ID": "admin",
    "X-Expected-Organization-ID": "org",
    cookie: "listingkit_workbench_org=org",
  };
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  expect(
    (
      await proxyMembers(
        new Request("http://localhost:3000/api/account/members/invitations", {
          method: "POST",
          headers,
        }),
        "token",
        "admin",
      )
    ).status,
  ).toBe(400);
  expect(fetch).not.toHaveBeenCalled();
});
