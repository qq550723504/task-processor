import { afterEach, expect, it, vi } from "vitest";
import { saveEnterpriseRole } from "./enterprise-roles";
const key = "e5836e4e-9c13-4df4-a67d-4c4e896b941a";
const id = "sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_01";
afterEach(() => vi.unstubAllGlobals());
it("rejects a save receipt for another role without changing the original intent", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValue(
      Response.json({
        schemaVersion: "enterprise-role-mutation-v1",
        userId: "actor",
        organizationId: "org",
        operationId: key,
        role: {
          id: id.slice(0, -2) + "02",
          name: "other",
          modules: [],
          version: 2,
          system: false,
        },
      }),
    );
  vi.stubGlobal("fetch", fetch);
  await expect(
    saveEnterpriseRole(
      { expectedUserId: "actor", expectedOrganizationId: "org" },
      key,
      id,
      { modules: [], expectedVersion: 1 },
    ),
  ).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
  expect(fetch.mock.calls[0][1].headers.get("Idempotency-Key")).toBe(key);
});
