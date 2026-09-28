import { afterEach, expect, it, vi } from "vitest";
import {
  getAccountFacts,
  getAccountSessionAuthentication,
  getAccountPreferences,
  saveAccountPreferences,
} from "./account-facts";

afterEach(() => vi.unstubAllGlobals());

it("binds authentication time to the signed-in subject and keeps missing time explicit", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ ok: true, identity: { userId: "u1", tenantId: "org-A" }, authenticatedAt: "2026-09-01T02:03:04.000Z" }))
    .mockResolvedValueOnce(Response.json({ ok: true, identity: { userId: "other" }, authenticatedAt: "2026-09-01T02:03:04.000Z" }))
    .mockResolvedValueOnce(Response.json({ ok: true, identity: { userId: "u1" }, authenticatedAt: null }));
  vi.stubGlobal("fetch", fetcher);
  await expect(getAccountSessionAuthentication({ expectedUserId: "u1" })).resolves.toEqual({ userId: "u1", authenticatedAt: "2026-09-01T02:03:04.000Z" });
  expect(fetcher).toHaveBeenCalledWith("/api/zitadel-auth/session", expect.objectContaining({ method: "GET", cache: "no-store", redirect: "error", credentials: "same-origin" }));
  await expect(getAccountSessionAuthentication({ expectedUserId: "u1" })).rejects.toMatchObject({ code: "IDENTITY_CONTEXT_CHANGED" });
  await expect(getAccountSessionAuthentication({ expectedUserId: "u1" })).resolves.toEqual({ userId: "u1", authenticatedAt: null });
});

it.each(["now", 1788228000])("rejects malformed session authentication time %s", async authenticatedAt => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ok: true, identity: { userId: "u1" }, authenticatedAt })));
  await expect(getAccountSessionAuthentication({ expectedUserId: "u1" })).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});
it("reads real identity dates without leaking another subject", async () => {
  const payload = {
    schemaVersion: "account-identity-facts-v1",
    userId: "u1",
    registeredAt: "2026-09-01T02:03:04Z",
    lastLogin: null,
    passwordChangedAt: null,
    source: "zitadel_auth_v1",
    readAt: "2026-09-28T01:00:00Z",
  };
  const fetcher = vi
    .fn()
    .mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify(payload), {
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
  vi.stubGlobal("fetch", fetcher);
  await expect(getAccountFacts({ expectedUserId: "u1" })).resolves.toEqual(
    payload,
  );
  expect(fetcher).toHaveBeenCalledWith(
    "/api/account/facts",
    expect.objectContaining({
      method: "GET",
      cache: "no-store",
      redirect: "error",
    }),
  );
  await expect(getAccountFacts({ expectedUserId: "u2" })).rejects.toMatchObject(
    { code: "IDENTITY_CONTEXT_CHANGED" },
  );
});
it("persists region input and keeps a legitimate empty region", async () => {
  const payload = {
    schemaVersion: "account-preferences-v1",
    userId: "u1",
    country: "",
    province: "",
    city: "",
    updatedAt: null,
    readAt: "2026-09-28T01:00:00Z",
    source: "account_profile",
  };
  const fetcher = vi
    .fn()
    .mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify(payload), {
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
  vi.stubGlobal("fetch", fetcher);
  await expect(
    getAccountPreferences({ expectedUserId: "u1" }),
  ).resolves.toEqual(payload);
  await saveAccountPreferences({
    expectedUserId: "u1",
    input: { country: "", province: "", city: "" },
  });
  expect(fetcher).toHaveBeenLastCalledWith(
    "/api/account/preferences",
    expect.objectContaining({
      method: "PUT",
      body: JSON.stringify({ country: "", province: "", city: "" }),
    }),
  );
  await expect(
    saveAccountPreferences({
      expectedUserId: "u1",
      input: { country: "bad\nvalue", province: "", city: "" },
    }),
  ).rejects.toMatchObject({ code: "INVALID_REQUEST" });
  expect(fetcher).toHaveBeenCalledTimes(2);
});
