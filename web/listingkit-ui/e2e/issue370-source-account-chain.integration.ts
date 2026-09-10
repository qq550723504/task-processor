import { readFile, writeFile } from "node:fs/promises";

import { afterAll, beforeAll, expect, it, vi } from "vitest";

import {
  createSourceAccount,
  disableSourceAccount,
  enableSourceAccount,
  getSourceAccount,
  listSourceAccounts,
  type SourceAccountAPIError,
} from "@/lib/api/source-accounts";

type FixtureManifest = {
  origin: string;
  goOrigin: string;
  sourceHead: string;
  goHead: string;
  evidencePath: string;
  sessions: Record<string, { cookie: string; subject: string }>;
};

type Observation = {
  resources: number;
  operations: number;
  businessPosts: number;
  canceled: number;
  tables: string[];
};

const manifestPath = process.env.SOURCE_ACCOUNT_FIXTURE_MANIFEST;
if (!manifestPath) {
  throw new Error(
    "Run through scripts/source-account-final-acceptance.mjs with a task-owned fixture",
  );
}

const manifest = JSON.parse(
  await readFile(manifestPath, "utf8"),
) as FixtureManifest;
const nativeFetch = globalThis.fetch;
let sessionName = "actor-a";
let selectedOrganization = "org-b";
const report: Array<Record<string, unknown>> = [];

const keys = {
  primary: "0198c5c0-1001-7001-8001-000000000001",
  disable: "0198c5c0-1002-7002-8002-000000000002",
  enable: "0198c5c0-1003-7003-8003-000000000003",
  revoke: "0198c5c0-1004-7004-8004-000000000004",
  lost: "0198c5c0-1005-7005-8005-000000000005",
  canceled: "0198c5c0-1006-7006-8006-000000000006",
  timedOut: "0198c5c0-1007-7007-8007-000000000007",
} as const;

beforeAll(() => {
  vi.stubGlobal(
    "fetch",
    (input: RequestInfo | URL, init: RequestInit = {}) => {
      if (typeof input !== "string" || !input.startsWith("/")) {
        return nativeFetch(input, init);
      }
      const headers = new Headers(init.headers);
      headers.set(
        "Cookie",
        `${manifest.sessions[sessionName]!.cookie}; shuomi_effective_organization=${selectedOrganization}`,
      );
      headers.set("Origin", manifest.origin);
      headers.set("Sec-Fetch-Site", "same-origin");
      headers.set("Authorization", "Bearer browser-forged");
      headers.set("X-Requested-Organization-ID", "forged-browser-org");
      headers.set("X-Tenant-ID", "forged-legacy-org");
      headers.set("X-User-ID", "forged-browser-user");
      return nativeFetch(new URL(input, manifest.origin), { ...init, headers });
    },
  );
});

afterAll(() => vi.unstubAllGlobals());

function actor() {
  return manifest.sessions[sessionName]!.subject;
}

function select(name: string, organization: string) {
  sessionName = name;
  selectedOrganization = organization;
}

function create(
  displayName: string,
  idempotencyKey: string,
  options: { expectedActor?: string; expectedOrganization?: string; signal?: AbortSignal } = {},
) {
  return createSourceAccount({
    displayName,
    platform: "1688",
    idempotencyKey,
    expectedActorSubject: options.expectedActor ?? actor(),
    expectedOrganizationId:
      options.expectedOrganization ?? selectedOrganization,
    signal: options.signal,
  });
}

async function control(path: string, payload?: unknown) {
  const response = await nativeFetch(`${manifest.goOrigin}/fixture/${path}`, {
    method: payload === undefined ? "GET" : "POST",
    headers:
      payload === undefined ? undefined : { "Content-Type": "application/json" },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  expect(response.ok, `${path}: ${await response.clone().text()}`).toBe(true);
  return response;
}

async function observe() {
  return (await (await control("observe")).json()) as Observation;
}

async function activity() {
  return (await (await control("activity")).json()) as Pick<
    Observation,
    "businessPosts" | "canceled"
  >;
}

async function eventually(
  check: () => Promise<boolean>,
  label: string,
  timeout = 5_000,
) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    if (await check()) return;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error(`Timed out waiting for ${label}`);
}

async function rejected(
  name: string,
  operation: Promise<unknown>,
  expected: Partial<SourceAccountAPIError>,
) {
  await expect(operation, name).rejects.toMatchObject(expected);
  report.push({ name, code: expected.code, outcome: expected.outcome });
}

it("executes the actual Source Account client -> Next/Auth.js -> Go -> empty PostgreSQL chain", async () => {
  const empty = await observe();
  expect(empty).toMatchObject({
    resources: 0,
    operations: 0,
    tables: [
      "goose_source_account_registry_version",
      "source_account_operations",
      "source_account_resources",
    ],
  });

  const primary = await create("Primary 1688", keys.primary);
  expect(primary).toMatchObject({
    replayed: false,
    etag: '"1"',
    account: {
      platform: "1688",
      displayName: "Primary 1688",
      managementStatus: "enabled",
      connectionStatus: "pending_connection",
      version: "1",
    },
  });
  report.push({ name: "register", id: primary.account.id, status: "created" });

  const page = await listSourceAccounts({ expectedOrganizationId: "org-b" });
  expect(page.items.map((item) => item.id)).toEqual([primary.account.id]);
  const detail = await getSourceAccount({
    sourceAccountId: primary.account.id,
    expectedOrganizationId: "org-b",
  });
  expect(detail).toMatchObject({ etag: '"1"', account: primary.account });

  select("viewer", "org-b");
  await rejected(
    "read-only role cannot register",
    create("Viewer denied", keys.primary),
    { status: 403, code: "PERMISSION_DENIED", outcome: "rejected" },
  );
  select("actor-a", "org-b");

  select("actor-a", "org-c");
  expect(
    (await listSourceAccounts({ expectedOrganizationId: "org-c" })).items,
  ).toEqual([]);
  await rejected(
    "cross-organization detail is hidden",
    getSourceAccount({
      sourceAccountId: primary.account.id,
      expectedOrganizationId: "org-c",
    }),
    { status: 404, code: "SOURCE_ACCOUNT_NOT_FOUND", outcome: "rejected" },
  );
  select("actor-a", "org-b");

  const replay = await create("Primary 1688", keys.primary);
  expect(replay).toMatchObject({ replayed: true, account: primary.account });
  await rejected(
    "same key with a different payload conflicts",
    create("Changed payload", keys.primary),
    { status: 409, code: "IDEMPOTENCY_CONFLICT", outcome: "rejected" },
  );

  const disabled = await disableSourceAccount({
    sourceAccountId: primary.account.id,
    ifMatch: primary.etag,
    idempotencyKey: keys.disable,
    expectedActorSubject: actor(),
    expectedOrganizationId: selectedOrganization,
  });
  expect(disabled).toMatchObject({
    replayed: false,
    etag: '"2"',
    account: { managementStatus: "disabled", version: "2" },
  });
  const enabled = await enableSourceAccount({
    sourceAccountId: primary.account.id,
    ifMatch: disabled.etag,
    idempotencyKey: keys.enable,
    expectedActorSubject: actor(),
    expectedOrganizationId: selectedOrganization,
  });
  expect(enabled).toMatchObject({
    replayed: false,
    etag: '"3"',
    account: { managementStatus: "enabled", version: "3" },
  });

  await control("revoke", {});
  await rejected(
    "live revocation blocks lifecycle write",
    disableSourceAccount({
      sourceAccountId: primary.account.id,
      ifMatch: enabled.etag,
      idempotencyKey: keys.revoke,
      expectedActorSubject: actor(),
      expectedOrganizationId: selectedOrganization,
    }),
    { status: 403, code: "ORGANIZATION_ACCESS_DENIED", outcome: "rejected" },
  );
  await control("restore", {});
  const disabledAfterRestore = await disableSourceAccount({
    sourceAccountId: primary.account.id,
    ifMatch: enabled.etag,
    idempotencyKey: keys.revoke,
    expectedActorSubject: actor(),
    expectedOrganizationId: selectedOrganization,
  });
  expect(disabledAfterRestore.account).toMatchObject({
    managementStatus: "disabled",
    version: "4",
  });
  const currentReplay = await create("Primary 1688", keys.primary);
  expect(currentReplay).toMatchObject({
    replayed: true,
    account: { managementStatus: "disabled", version: "4" },
  });

  await control("drop-next", {});
  await rejected(
    "committed response loss remains unknown",
    create("Lost response", keys.lost),
    { status: 503, code: "OUTCOME_UNKNOWN", outcome: "unknown" },
  );
  const committedUnknown = await observe();
  expect(committedUnknown).toMatchObject({ resources: 2, operations: 5 });

  select("actor-c", "org-b");
  const beforeActorMismatch = await observe();
  await rejected(
    "a changed actor cannot verify the original intent",
    create("Lost response", keys.lost, { expectedActor: "actor-a" }),
    { status: 409, code: "IDENTITY_CONTEXT_CHANGED", outcome: "rejected" },
  );
  expect(await observe()).toMatchObject(beforeActorMismatch);

  select("actor-a", "org-c");
  const beforeOrganizationMismatch = await observe();
  await rejected(
    "a changed organization cannot verify the original intent",
    create("Lost response", keys.lost, { expectedOrganization: "org-b" }),
    { status: 409, code: "ORGANIZATION_CONTEXT_CHANGED", outcome: "rejected" },
  );
  expect(await observe()).toMatchObject(beforeOrganizationMismatch);

  select("actor-a", "org-b");
  await control("restart", {});
  const recovered = await create("Lost response", keys.lost);
  expect(recovered).toMatchObject({ replayed: true });
  expect((await observe()).operations).toBe(5);

  const beforeCancel = await observe();
  await control("lock", { key: keys.canceled });
  const beforeCancelActivity = await activity();
  const cancel = new AbortController();
  const canceled = create("Canceled request", keys.canceled, {
    signal: cancel.signal,
  });
  await eventually(
    async () =>
      (await activity()).businessPosts > beforeCancelActivity.businessPosts,
    "the canceled write to reach Go",
  );
  cancel.abort();
  await rejected("caller cancellation is conservatively unknown", canceled, {
    status: 0,
    code: "OUTCOME_UNKNOWN",
    outcome: "unknown",
  });
  await eventually(
    async () => (await activity()).canceled > beforeCancelActivity.canceled,
    "Go cancellation",
  );
  await control("release", {});
  expect(await observe()).toMatchObject({
    resources: beforeCancel.resources,
    operations: beforeCancel.operations,
  });
  const afterCancel = await create("Canceled request", keys.canceled);
  expect(afterCancel).toMatchObject({ replayed: false });

  const beforeTimeout = await observe();
  await control("lock", { key: keys.timedOut });
  await rejected(
    "Go deadline after dispatch is unknown",
    create("Timed out request", keys.timedOut),
    { status: 504, code: "DEADLINE_EXCEEDED", outcome: "unknown" },
  );
  await control("release", {});
  expect(await observe()).toMatchObject({
    resources: beforeTimeout.resources,
    operations: beforeTimeout.operations,
  });
  const afterTimeout = await create("Timed out request", keys.timedOut);
  expect(afterTimeout).toMatchObject({ replayed: false });

  const final = await observe();
  expect(final).toMatchObject({ resources: 4, operations: 7 });
  const firstPage = await listSourceAccounts({
    expectedOrganizationId: "org-b",
    limit: 1,
  });
  expect(firstPage.items).toHaveLength(1);
  expect(firstPage.nextCursor).not.toBeNull();
  const secondPage = await listSourceAccounts({
    expectedOrganizationId: "org-b",
    limit: 100,
    cursor: firstPage.nextCursor!,
  });
  expect(secondPage.items).toHaveLength(3);
  expect(secondPage.nextCursor).toBeNull();

  await writeFile(
    manifest.evidencePath,
    JSON.stringify(
      {
        passed: true,
        sourceHead: manifest.sourceHead,
        goHead: manifest.goHead,
        assertions: report,
        final,
        actual: [
          "Source Account TypeScript client",
          "Next server and Auth.js encrypted session",
          "Workbench BFF transport",
          "Go verified identity and Effective Organization middleware",
          "Go permission, service, Unit of Work, and repository",
          "task-owned empty PostgreSQL initialized by the schema CLI",
        ],
        externalSubstitutes: ["external identity and grant issuance"],
        notRun: [
          "real ZITADEL",
          "real 1688 authentication or crawling",
          "shared or production composition",
          "deployment",
        ],
      },
      null,
      2,
    ),
  );
});
