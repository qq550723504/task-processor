import { readFile, writeFile } from "node:fs/promises";

import { expect, it, vi } from "vitest";

import {
  createSourceAccount,
  disableSourceAccount,
  enableSourceAccount,
  getSourceAccount,
  listSourceAccounts,
} from "@/lib/api/source-accounts";

type Manifest = {
  origin: string;
  goOrigin: string;
  phase: "create" | "revoked" | "verify";
  statePath: string;
  evidencePath: string;
  organizations: { B: string; C: string };
  sessions: Record<"admin" | "viewer", { cookie: string; subject: string }>;
};

const path = process.env.CURRENT_APPLICATION_CHAIN_MANIFEST;
if (!path) throw new Error("CURRENT_APPLICATION_CHAIN_MANIFEST is required");
const manifest = JSON.parse(await readFile(path, "utf8")) as Manifest;
const nativeFetch = globalThis.fetch;
let actor: "admin" | "viewer" = "admin";
let organization = manifest.organizations.B;
let loseResponseForKey: string | null = null;

vi.stubGlobal("fetch", async (input: RequestInfo | URL, init: RequestInit = {}) => {
  if (typeof input !== "string" || !input.startsWith("/")) return nativeFetch(input, init);
  const headers = new Headers(init.headers);
  headers.set("Cookie", `${manifest.sessions[actor].cookie}; shuomi_effective_organization=${organization}`);
  headers.set("Origin", manifest.origin);
  headers.set("Sec-Fetch-Site", "same-origin");
  headers.set("Authorization", "Bearer browser-forged-and-ignored");
  headers.set("X-Requested-Organization-ID", "browser-forged-and-ignored");
  const response = await nativeFetch(new URL(input, manifest.origin), { ...init, headers });
  if (loseResponseForKey && headers.get("Idempotency-Key") === loseResponseForKey) {
    loseResponseForKey = null;
    await response.arrayBuffer();
    throw new TypeError("simulated response loss after the actual BFF response completed");
  }
  return response;
});

const createKey = "01991e24-1001-7001-8001-000000000001";
const disableKey = "01991e24-1002-7002-8002-000000000002";
const enableKey = "01991e24-1003-7003-8003-000000000003";
const staleKey = "01991e24-1004-7004-8004-000000000004";
const finalDisableKey = "01991e24-1005-7005-8005-000000000005";
const secondCreateKey = "01991e24-1006-7006-8006-000000000006";
const thirdCreateKey = "01991e24-1007-7007-8007-000000000007";
const revokedKey = "01991e24-1008-7008-8008-000000000008";

function create() {
  return createSourceAccount({
    displayName: "RUN-1 1688",
    platform: "1688",
    idempotencyKey: createKey,
    expectedActorSubject: manifest.sessions.admin.subject,
    expectedOrganizationId: manifest.organizations.B,
  });
}

it("uses the actual SA2 client and BFF against the normal current application", async () => {
  if (manifest.phase === "revoked") {
    const state = JSON.parse(await readFile(manifest.statePath, "utf8")) as { account: { id: string }; etag: string };
    try {
      await disableSourceAccount({ sourceAccountId: state.account.id, ifMatch: state.etag, idempotencyKey: revokedKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B });
      throw new Error("revoked write was accepted");
    } catch (error) {
      expect(error).toMatchObject({ status: 403, outcome: "rejected" });
      expect(["ORGANIZATION_ACCESS_DENIED", "ORGANIZATION_ACCESS_REVOKED"]).toContain((error as { code: string }).code);
    }
    return;
  }
  if (manifest.phase === "create") {
    expect((await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B })).items).toEqual([]);
    const created = await create();
    expect(created).toMatchObject({ replayed: false, etag: '"1"', account: { platform: "1688", managementStatus: "enabled", version: "1" } });
    expect(await create()).toMatchObject({ replayed: true, account: { id: created.account.id } });
    await expect(createSourceAccount({ displayName: "Different payload", platform: "1688", idempotencyKey: createKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B })).rejects.toMatchObject({ status: 409, code: "IDEMPOTENCY_CONFLICT", outcome: "rejected" });
    expect((await getSourceAccount({ sourceAccountId: created.account.id, expectedOrganizationId: manifest.organizations.B })).account.id).toBe(created.account.id);

    organization = manifest.organizations.C;
    expect((await listSourceAccounts({ expectedOrganizationId: manifest.organizations.C })).items).toEqual([]);
    await expect(getSourceAccount({ sourceAccountId: created.account.id, expectedOrganizationId: manifest.organizations.C })).rejects.toMatchObject({ status: 404, code: "SOURCE_ACCOUNT_NOT_FOUND" });

    organization = manifest.organizations.B;
    actor = "viewer";
    expect((await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B })).items).toHaveLength(1);
    await expect(createSourceAccount({ displayName: "Viewer denied", platform: "1688", idempotencyKey: createKey, expectedActorSubject: manifest.sessions.viewer.subject, expectedOrganizationId: manifest.organizations.B })).rejects.toMatchObject({ status: 403, code: "PERMISSION_DENIED", outcome: "rejected" });

    actor = "admin";
    const disabled = await disableSourceAccount({ sourceAccountId: created.account.id, ifMatch: created.etag, idempotencyKey: disableKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B });
    const enabled = await enableSourceAccount({ sourceAccountId: created.account.id, ifMatch: disabled.etag, idempotencyKey: enableKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B });
    expect(enabled).toMatchObject({ replayed: false, etag: '"3"', account: { managementStatus: "enabled", version: "3" } });
    await expect(disableSourceAccount({ sourceAccountId: created.account.id, ifMatch: '"1"', idempotencyKey: staleKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B })).rejects.toMatchObject({ status: 409, code: "VERSION_CONFLICT", outcome: "rejected" });
    const final = await disableSourceAccount({ sourceAccountId: created.account.id, ifMatch: enabled.etag, idempotencyKey: finalDisableKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B });
    expect(await create()).toMatchObject({ replayed: true, account: { id: created.account.id, managementStatus: "disabled", version: "4" } });
    expect(await enableSourceAccount({ sourceAccountId: created.account.id, ifMatch: disabled.etag, idempotencyKey: enableKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B })).toMatchObject({ replayed: true, account: { managementStatus: "disabled", version: "4" } });
    loseResponseForKey = secondCreateKey;
    await expect(createSourceAccount({ displayName: "RUN-1 page 2", platform: "1688", idempotencyKey: secondCreateKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B })).rejects.toMatchObject({ status: 0, code: "OUTCOME_UNKNOWN", outcome: "unknown" });
    expect(await createSourceAccount({ displayName: "RUN-1 page 2", platform: "1688", idempotencyKey: secondCreateKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B })).toMatchObject({ replayed: true, account: { managementStatus: "enabled", version: "1" } });
    await expect(createSourceAccount({ displayName: "Changed after response loss", platform: "1688", idempotencyKey: secondCreateKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B })).rejects.toMatchObject({ status: 409, code: "IDEMPOTENCY_CONFLICT", outcome: "rejected" });
    await createSourceAccount({ displayName: "RUN-1 page 3", platform: "1688", idempotencyKey: thirdCreateKey, expectedActorSubject: manifest.sessions.admin.subject, expectedOrganizationId: manifest.organizations.B });
    const firstPage = await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B, limit: 1 });
    expect(firstPage.items).toHaveLength(1);
    expect(firstPage.nextCursor).not.toBeNull();
    const nextPage = await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B, limit: 100, cursor: firstPage.nextCursor! });
    expect(nextPage.items).toHaveLength(2);
    await writeFile(manifest.statePath, JSON.stringify({ account: final.account, etag: final.etag }), { mode: 0o600 });
  } else {
    const state = JSON.parse(await readFile(manifest.statePath, "utf8")) as { account: { id: string }; etag: string };
    const page = await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B });
    expect(page.items.map((item) => item.id)).toContain(state.account.id);
    expect(page.items).toHaveLength(3);
    expect(await create()).toMatchObject({ replayed: true, account: { id: state.account.id, version: "4", managementStatus: "disabled" } });
    expect((await getSourceAccount({ sourceAccountId: state.account.id, expectedOrganizationId: manifest.organizations.B })).etag).toBe(state.etag);
    await writeFile(manifest.evidencePath, JSON.stringify({ passed: true, phase: manifest.phase, accountId: state.account.id, persistedVersion: "4", resourceCount: 3, actual: ["Source Account TypeScript client", "Next/Auth.js real provider session", "Workbench BFF", "normal Go current-application binary", "task-owned PostgreSQL"] }, null, 2));
  }
});
