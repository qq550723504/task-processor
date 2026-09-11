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
  phase: "create" | "verify";
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

vi.stubGlobal("fetch", (input: RequestInfo | URL, init: RequestInit = {}) => {
  if (typeof input !== "string" || !input.startsWith("/")) return nativeFetch(input, init);
  const headers = new Headers(init.headers);
  headers.set("Cookie", `${manifest.sessions[actor].cookie}; shuomi_effective_organization=${organization}`);
  headers.set("Origin", manifest.origin);
  headers.set("Sec-Fetch-Site", "same-origin");
  headers.set("Authorization", "Bearer browser-forged-and-ignored");
  headers.set("X-Requested-Organization-ID", "browser-forged-and-ignored");
  return nativeFetch(new URL(input, manifest.origin), { ...init, headers });
});

const createKey = "01991e24-1001-7001-8001-000000000001";
const disableKey = "01991e24-1002-7002-8002-000000000002";
const enableKey = "01991e24-1003-7003-8003-000000000003";

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
  if (manifest.phase === "create") {
    expect((await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B })).items).toEqual([]);
    const created = await create();
    expect(created).toMatchObject({ replayed: false, etag: '"1"', account: { platform: "1688", managementStatus: "enabled", version: "1" } });
    expect(await create()).toMatchObject({ replayed: true, account: { id: created.account.id } });
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
    await writeFile(manifest.statePath, JSON.stringify({ account: enabled.account, etag: enabled.etag }), { mode: 0o600 });
  } else {
    const state = JSON.parse(await readFile(manifest.statePath, "utf8")) as { account: { id: string }; etag: string };
    const page = await listSourceAccounts({ expectedOrganizationId: manifest.organizations.B });
    expect(page.items.map((item) => item.id)).toEqual([state.account.id]);
    expect(await create()).toMatchObject({ replayed: true, account: { id: state.account.id, version: "3" } });
    expect((await getSourceAccount({ sourceAccountId: state.account.id, expectedOrganizationId: manifest.organizations.B })).etag).toBe(state.etag);
    await writeFile(manifest.evidencePath, JSON.stringify({ passed: true, phase: manifest.phase, accountId: state.account.id, persistedVersion: "3", actual: ["Source Account TypeScript client", "Next/Auth.js real provider session", "Workbench BFF", "normal Go current-application binary", "task-owned PostgreSQL"] }, null, 2));
  }
});
