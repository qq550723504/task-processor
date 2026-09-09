import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  SourceAccountAPIError,
  createSourceAccount,
  disableSourceAccount,
  enableSourceAccount,
  getSourceAccount,
  listSourceAccounts,
} from "@/lib/api/source-accounts";

const ACCOUNT_ID = "018f1f0e-7b5d-7c3a-8a11-1234567890ab";
const OTHER_ACCOUNT_ID = "018f1f0e-7b5d-7c3a-8a11-1234567890ac";
const IDEMPOTENCY_KEY = "22222222-2222-4222-8222-22222222222b";
const ORGANIZATION_ID = "org-b";
const ACTOR_ID = "user-a";
const MAX_VERSION = "9223372036854775807";
const account = {
  id: ACCOUNT_ID,
  platform: "1688" as const,
  displayName: "Primary 1688",
  managementStatus: "enabled" as const,
  connectionStatus: "pending_connection" as const,
  version: MAX_VERSION,
  createdAt: "2026-09-09T01:02:03Z",
  updatedAt: "2026-09-09T02:03:04Z",
};

const fetchMock = vi.fn<typeof fetch>();

function response(payload: unknown, status = 200, etag?: string) {
  const headers = new Headers({ "Content-Type": "application/json" });
  if (etag) headers.set("ETag", etag);
  return new Response(JSON.stringify(payload), { status, headers });
}

function requestAt(index = 0) {
  const [input, init] = fetchMock.mock.calls[index]!;
  return { input, init: init!, headers: new Headers(init?.headers) };
}

describe("Source Account browser API", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  it("sends one frozen Create intent and preserves the exact strong ETag", async () => {
    fetchMock.mockResolvedValueOnce(
      response({ schemaVersion: 1, account, replayed: false }, 201, `"${MAX_VERSION}"`),
    );

    await expect(
      createSourceAccount({
        displayName: account.displayName,
        platform: "1688",
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
      }),
    ).resolves.toEqual({
      schemaVersion: 1,
      account,
      replayed: false,
      etag: `"${MAX_VERSION}"`,
    });

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(requestAt().input).toBe("/api/workbench/source-accounts");
    expect(requestAt().init).toMatchObject({
      method: "POST",
      credentials: "same-origin",
      body: JSON.stringify({ displayName: account.displayName, platform: "1688" }),
    });
    expect([...requestAt().headers]).toEqual([
      ["accept", "application/json"],
      ["content-type", "application/json"],
      ["idempotency-key", IDEMPOTENCY_KEY],
      ["x-expected-organization-id", ORGANIZATION_ID],
      ["x-expected-user-id", ACTOR_ID],
    ]);
  });

  it("builds bounded list and detail reads without mutation headers", async () => {
    fetchMock
      .mockResolvedValueOnce(
        response({ schemaVersion: 1, items: [account], nextCursor: "eyJ2IjoxfQ" }),
      )
      .mockResolvedValueOnce(
        response({ schemaVersion: 1, account }, 200, `"${MAX_VERSION}"`),
      );

    await expect(
      listSourceAccounts({
        limit: 20,
        cursor: "eyJ2IjoxfQ",
        expectedOrganizationId: ORGANIZATION_ID,
      }),
    ).resolves.toEqual({ schemaVersion: 1, items: [account], nextCursor: "eyJ2IjoxfQ" });
    await expect(
      getSourceAccount({ sourceAccountId: ACCOUNT_ID, expectedOrganizationId: ORGANIZATION_ID }),
    ).resolves.toEqual({ schemaVersion: 1, account, etag: `"${MAX_VERSION}"` });

    expect(requestAt(0).input).toBe(
      "/api/workbench/source-accounts?limit=20&cursor=eyJ2IjoxfQ",
    );
    expect([...requestAt(0).headers]).toEqual([
      ["accept", "application/json"],
      ["x-expected-organization-id", ORGANIZATION_ID],
    ]);
    expect(requestAt(1).input).toBe(`/api/workbench/source-accounts/${ACCOUNT_ID}`);
  });

  it("rejects a non-canonical cursor before fetch", async () => {
    await expect(
      listSourceAccounts({
        cursor: "next-token",
        expectedOrganizationId: ORGANIZATION_ID,
      }),
    ).rejects.toMatchObject({ code: "INVALID_REQUEST", outcome: "not_sent" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it.each([
    ["enable", enableSourceAccount],
    ["disable", disableSourceAccount],
  ] as const)("sends the exact %s lifecycle intent without deriving an ETag", async (action, invoke) => {
    fetchMock.mockResolvedValueOnce(
      response({ schemaVersion: 1, account, replayed: false }, 200, `"${MAX_VERSION}"`),
    );

    await invoke({
      sourceAccountId: ACCOUNT_ID,
      ifMatch: `"${MAX_VERSION}"`,
      idempotencyKey: IDEMPOTENCY_KEY,
      expectedOrganizationId: ORGANIZATION_ID,
      expectedActorSubject: ACTOR_ID,
    });

    expect(requestAt().input).toBe(`/api/workbench/source-accounts/${ACCOUNT_ID}/${action}`);
    expect(requestAt().init.body).toBeUndefined();
    expect(requestAt().headers.get("If-Match")).toBe(`"${MAX_VERSION}"`);
    expect(requestAt().headers.get("Idempotency-Key")).toBe(IDEMPOTENCY_KEY);
  });

  it.each([
    { displayName: " Primary 1688" },
    { displayName: "x".repeat(121) },
    { displayName: "bad\u0000name" },
    { displayName: "bad\ud800" },
    { displayName: "bad\udc00" },
    { expectedActorSubject: " user-a" },
    { idempotencyKey: "00000000-0000-0000-0000-000000000000" },
    { owner: "user-a" },
  ])("rejects invalid mutation input before fetch: %o", async (replacement) => {
    await expect(
      createSourceAccount({
        displayName: account.displayName,
        platform: "1688",
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
        ...replacement,
      }),
    ).rejects.toMatchObject({ code: "INVALID_REQUEST", outcome: "not_sent" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("does not dispatch an already-aborted mutation", async () => {
    const controller = new AbortController();
    controller.abort();
    await expect(
      createSourceAccount({
        displayName: account.displayName,
        platform: "1688",
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
        signal: controller.signal,
      }),
    ).rejects.toMatchObject({ outcome: "not_sent" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("never retries a mutation and classifies transport loss after dispatch as unknown", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("network secret"));
    await expect(
      createSourceAccount({
        displayName: account.displayName,
        platform: "1688",
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
      }),
    ).rejects.toEqual(
      expect.objectContaining({
        name: "SourceAccountAPIError",
        code: "OUTCOME_UNKNOWN",
        outcome: "unknown",
      }),
    );
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it.each([
    [502, "DEPENDENCY_UNAVAILABLE", "unknown"],
    [502, "INVALID_UPSTREAM_RESPONSE", "unknown"],
    [503, "DEPENDENCY_UNAVAILABLE", "rejected"],
    [503, "OUTCOME_UNKNOWN", "unknown"],
    [504, "DEADLINE_EXCEEDED", "unknown"],
    [409, "IDENTITY_CONTEXT_CHANGED", "rejected"],
  ] as const)("classifies %i %s as %s", async (status, code, outcome) => {
    fetchMock.mockResolvedValueOnce(
      response({ code, message: "safe", requestId: "req-1", fieldErrors: [] }, status),
    );
    await expect(
      createSourceAccount({
        displayName: account.displayName,
        platform: "1688",
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
      }),
    ).rejects.toMatchObject({ status, code, requestId: "req-1", outcome });
  });

  it("rejects malformed mutation success as unknown and detail ID drift as a read failure", async () => {
    fetchMock
      .mockResolvedValueOnce(response({ schemaVersion: 1, account, replayed: false }, 201))
      .mockResolvedValueOnce(
        response(
          { schemaVersion: 1, account: { ...account, id: OTHER_ACCOUNT_ID } },
          200,
          `"${MAX_VERSION}"`,
        ),
      );

    await expect(
      createSourceAccount({
        displayName: account.displayName,
        platform: "1688",
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
      }),
    ).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE", outcome: "unknown" });
    await expect(
      getSourceAccount({ sourceAccountId: ACCOUNT_ID, expectedOrganizationId: ORGANIZATION_ID }),
    ).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE", outcome: "rejected" });
  });

  it("exposes the parsed safe error payload on SourceAccountAPIError", async () => {
    const payload = {
      code: "VERSION_CONFLICT",
      message: "Version changed",
      requestId: "req-2",
      fieldErrors: [{ field: "ifMatch", code: "conflict" }],
    };
    fetchMock.mockResolvedValueOnce(response(payload, 409));
    try {
      await disableSourceAccount({
        sourceAccountId: ACCOUNT_ID,
        ifMatch: `"${MAX_VERSION}"`,
        idempotencyKey: IDEMPOTENCY_KEY,
        expectedOrganizationId: ORGANIZATION_ID,
        expectedActorSubject: ACTOR_ID,
      });
      throw new Error("expected rejection");
    } catch (error) {
      expect(error).toBeInstanceOf(SourceAccountAPIError);
      expect(error).toMatchObject({ payload, outcome: "rejected" });
    }
  });
});
