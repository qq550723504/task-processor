import { afterEach, expect, it, vi } from "vitest";
import {
  getAccountFacts,
  getAccountPreferences,
  saveAccountPreferences,
} from "./account-facts";

afterEach(() => vi.unstubAllGlobals());
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
