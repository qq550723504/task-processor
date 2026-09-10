import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  buildWorkbenchBrowserResponse,
  buildWorkbenchUpstreamRequest,
} from "@/lib/server/workbench-proxy";

const ACCOUNT_ID = "018f1f0e-7b5d-7c3a-8a11-1234567890ab";
const OTHER_ACCOUNT_ID = "018f1f0e-7b5d-7c3a-8a11-1234567890ac";
const IDEMPOTENCY_KEY = "22222222-2222-4222-8222-22222222222b";
const UUIDV7_IDEMPOTENCY_KEY = "018f1f0e-7b5d-7c3a-8a11-1234567890ad";
const ORGANIZATION_ID = "org-b";
const ACTOR_ID = "user-a";
const MAX_VERSION = "9223372036854775807";
const account = {
  id: ACCOUNT_ID,
  platform: "1688",
  displayName: "Primary 1688",
  managementStatus: "enabled",
  connectionStatus: "pending_connection",
  version: MAX_VERSION,
  createdAt: "2026-09-09T01:02:03Z",
  updatedAt: "2026-09-09T02:03:04Z",
};

function browserRequest(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("cookie", `shuomi_effective_organization=${ORGANIZATION_ID}`);
  headers.set("X-Expected-Organization-ID", ORGANIZATION_ID);
  return new Request(`http://localhost/api/workbench/${path}`, { ...init, headers });
}

async function error(response: Response) {
  return { status: response.status, payload: await response.json() };
}

describe("Source Account Workbench BFF boundary", () => {
  beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost"));

  it("allowlists and canonicalizes the five routes only", async () => {
    const cases = [
      ["GET", "source-accounts?limit=20&cursor=next", "source-accounts?limit=20&cursor=next"],
      ["GET", `source-accounts/${ACCOUNT_ID}`, `source-accounts/${ACCOUNT_ID}`],
      ["POST", "source-accounts", "source-accounts"],
      ["POST", `source-accounts/${ACCOUNT_ID}/disable`, `source-accounts/${ACCOUNT_ID}/disable`],
      ["POST", `source-accounts/${ACCOUNT_ID}/enable`, `source-accounts/${ACCOUNT_ID}/enable`],
    ] as const;
    for (const [method, browserPath, upstreamPath] of cases) {
      const action = method === "POST" && browserPath !== "source-accounts";
      const request = browserRequest(browserPath, {
        method,
        headers: method === "POST" ? {
          Origin: "http://localhost",
          "Idempotency-Key": IDEMPOTENCY_KEY,
          "X-Expected-User-ID": ACTOR_ID,
          ...(action ? { "If-Match": `"${MAX_VERSION}"` } : {}),
          ...(!action ? { "Content-Type": "application/json" } : {}),
        } : undefined,
        body: method === "POST" && !action
          ? JSON.stringify({ displayName: account.displayName, platform: "1688" })
          : undefined,
      });
      const path = new URL(request.url).pathname.slice("/api/workbench/".length).split("/");
      const result = await buildWorkbenchUpstreamRequest(request, path, "server-token", ACTOR_ID);
      expect(result).not.toBeInstanceOf(Response);
      if (result instanceof Response) continue;
      expect(result.url).toBe(`http://localhost:8085/api/v1/workbench/${upstreamPath}`);
      const headers = new Headers(result.init.headers);
      expect(headers.get("Authorization")).toBe("Bearer server-token");
      expect(headers.get("X-Requested-Organization-ID")).toBe(ORGANIZATION_ID);
      expect(headers.has("X-Expected-User-ID")).toBe(false);
      expect(headers.has("X-Expected-Organization-ID")).toBe(false);
    }
  });

  it("rejects non-v7 IDs and all unlisted Source Account routes", async () => {
    for (const [method, path] of [
      ["GET", "source-accounts/11111111-1111-4111-8111-111111111111"],
      ["POST", `${ACCOUNT_ID}/verify`],
      ["DELETE", `source-accounts/${ACCOUNT_ID}`],
    ] as const) {
      const result = await buildWorkbenchUpstreamRequest(
        browserRequest(path, { method }),
        path.split("/"),
        "server-token",
        ACTOR_ID,
      );
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) expect(result.status).toBe(404);
    }
  });

  it.each([null, "", "user a", "USER-A", "user-a,user-b"])(
    "rejects invalid or mismatched actor assertion before forwarding: %s",
    async (assertion) => {
      const headers: Record<string, string> = {
        Origin: "http://localhost",
        "Content-Type": "application/json",
        "Idempotency-Key": IDEMPOTENCY_KEY,
      };
      if (assertion !== null) headers["X-Expected-User-ID"] = assertion;
      const result = await buildWorkbenchUpstreamRequest(
        browserRequest("source-accounts", {
          method: "POST",
          headers,
          body: JSON.stringify({ displayName: account.displayName, platform: "1688" }),
        }),
        ["source-accounts"],
        "server-token",
        ACTOR_ID,
      );
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) {
        expect(result.status).toBe(assertion === "USER-A" ? 409 : 400);
      }
    },
  );

  it("requires exact same-origin proof for mutation without trusting Host", async () => {
    for (const origin of [null, "https://attacker.example"]) {
      const headers: Record<string, string> = {
        Host: "localhost",
        "Content-Type": "application/json",
        "Idempotency-Key": IDEMPOTENCY_KEY,
        "X-Expected-User-ID": ACTOR_ID,
      };
      if (origin) headers.Origin = origin;
      const result = await buildWorkbenchUpstreamRequest(
        browserRequest("source-accounts", {
          method: "POST",
          headers,
          body: JSON.stringify({ displayName: account.displayName, platform: "1688" }),
        }),
        ["source-accounts"],
        "server-token",
        ACTOR_ID,
      );
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) expect(result.status).toBe(403);
    }
  });

  it("accepts a canonical RFC-variant UUIDv7 Idempotency-Key", async () => {
    const displayName = "Primary 🚀";
    const result = await buildWorkbenchUpstreamRequest(
      browserRequest("source-accounts", {
        method: "POST",
        headers: {
          Origin: "http://localhost",
          "Content-Type": "application/json",
          "Idempotency-Key": UUIDV7_IDEMPOTENCY_KEY,
          "X-Expected-User-ID": ACTOR_ID,
        },
        body: JSON.stringify({ displayName, platform: "1688" }),
      }),
      ["source-accounts"],
      "server-token",
      ACTOR_ID,
    );
    expect(result).not.toBeInstanceOf(Response);
    if (!(result instanceof Response)) {
      expect(new Headers(result.init.headers).get("Idempotency-Key")).toBe(
        UUIDV7_IDEMPOTENCY_KEY,
      );
      expect(result.init.body).toBe(JSON.stringify({ displayName, platform: "1688" }));
    }
  });

  it.each([
    ["duplicate field", '{"displayName":"a","displayName":"b","platform":"1688"}'],
    ["unknown field", '{"displayName":"a","platform":"1688","owner":"user-a"}'],
    ["untrimmed", '{"displayName":" a","platform":"1688"}'],
    ["unpaired high surrogate", '{"displayName":"bad\\ud800","platform":"1688"}'],
    ["unpaired low surrogate", '{"displayName":"bad\\udc00","platform":"1688"}'],
    ["wrong platform", '{"displayName":"a","platform":"other"}'],
  ])("rejects strict Create body: %s", async (_name, body) => {
    const result = await buildWorkbenchUpstreamRequest(
      browserRequest("source-accounts", {
        method: "POST",
        headers: {
          Origin: "http://localhost",
          "Content-Type": "application/json",
          "Idempotency-Key": IDEMPOTENCY_KEY,
          "X-Expected-User-ID": ACTOR_ID,
        },
        body,
      }),
      ["source-accounts"],
      "server-token",
      ACTOR_ID,
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it("preserves a signed-int64 strong If-Match without Number conversion", async () => {
    const request = browserRequest(`source-accounts/${ACCOUNT_ID}/disable`, {
      method: "POST",
      headers: {
        Origin: "http://localhost",
        "Idempotency-Key": IDEMPOTENCY_KEY,
        "If-Match": `"${MAX_VERSION}"`,
        "X-Expected-User-ID": ACTOR_ID,
      },
    });
    const result = await buildWorkbenchUpstreamRequest(
      request,
      ["source-accounts", ACCOUNT_ID, "disable"],
      "server-token",
      ACTOR_ID,
    );
    expect(result).not.toBeInstanceOf(Response);
    if (!(result instanceof Response)) {
      expect(new Headers(result.init.headers).get("If-Match")).toBe(`"${MAX_VERSION}"`);
    }
  });

  it("rejects non-canonical list cursors and duplicate Organization cookies", async () => {
    const invalidCursor = await buildWorkbenchUpstreamRequest(
      browserRequest("source-accounts?cursor=next-token", { method: "GET" }),
      ["source-accounts"],
      "server-token",
      ACTOR_ID,
    );
    expect(invalidCursor).toBeInstanceOf(Response);
    if (invalidCursor instanceof Response) expect(invalidCursor.status).toBe(400);

    const duplicateCookie = browserRequest("source-accounts", { method: "GET" });
    duplicateCookie.headers.set(
      "cookie",
      `shuomi_effective_organization=${ORGANIZATION_ID}; shuomi_effective_organization=${ORGANIZATION_ID}`,
    );
    const duplicateResult = await buildWorkbenchUpstreamRequest(
      duplicateCookie,
      ["source-accounts"],
      "server-token",
      ACTOR_ID,
    );
    expect(duplicateResult).toBeInstanceOf(Response);
    if (duplicateResult instanceof Response) {
      expect(duplicateResult.status).toBe(409);
      await expect(duplicateResult.json()).resolves.toMatchObject({
        code: "ORGANIZATION_CONTEXT_CHANGED",
      });
    }
  });

  it("validates source success, path identity, status, ETag and the 128 KiB limit", async () => {
    const valid = await buildWorkbenchBrowserResponse(
      new Response(JSON.stringify({ schemaVersion: 1, account }), {
        status: 200,
        headers: { "Content-Type": "application/json", ETag: `"${MAX_VERSION}"` },
      }),
      "source-account-detail",
      ACCOUNT_ID,
    );
    expect(valid.status).toBe(200);
    expect(valid.headers.get("ETag")).toBe(`"${MAX_VERSION}"`);

    for (const upstream of [
      new Response(JSON.stringify({ schemaVersion: 1, account: { ...account, id: OTHER_ACCOUNT_ID } }), {
        status: 200,
        headers: { "Content-Type": "application/json", ETag: `"${MAX_VERSION}"` },
      }),
      new Response(JSON.stringify({ schemaVersion: 1, account }), {
        status: 201,
        headers: { "Content-Type": "application/json", ETag: `"${MAX_VERSION}"` },
      }),
      new Response(JSON.stringify({ schemaVersion: 1, account }), {
        status: 200,
        headers: { "Content-Type": "application/json", ETag: '"2"' },
      }),
      new Response(JSON.stringify({ schemaVersion: 1, account, padding: "x".repeat(128 * 1024) }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    ]) {
      await expect(error(await buildWorkbenchBrowserResponse(
        upstream,
        "source-account-detail",
        ACCOUNT_ID,
      ))).resolves.toMatchObject({ status: 502, payload: { code: "INVALID_UPSTREAM_RESPONSE" } });
    }
  });

  it("maps malformed mutation responses to OUTCOME_UNKNOWN", async () => {
    const result = await buildWorkbenchBrowserResponse(
      new Response("not-json", { status: 200, headers: { "Content-Type": "text/plain" } }),
      "source-account-mutation",
      ACCOUNT_ID,
      { sourceMutation: true, requestId: "req-safe" },
    );
    await expect(error(result)).resolves.toEqual({
      status: 503,
      payload: {
        code: "OUTCOME_UNKNOWN",
        message: "Source Account mutation outcome is unknown",
        requestId: "req-safe",
        fieldErrors: [],
      },
    });
  });

  it("preserves only documented Source Account errors with their exact status", async () => {
    const documented = await buildWorkbenchBrowserResponse(
      new Response(
        JSON.stringify({
          code: "DEPENDENCY_UNAVAILABLE",
          message: "Unavailable",
          requestId: "req-1",
          fieldErrors: [],
        }),
        { status: 503, headers: { "Content-Type": "application/json" } },
      ),
      "source-account-mutation",
      ACCOUNT_ID,
      { sourceMutation: true, requestId: "req-safe" },
    );
    expect(documented.status).toBe(503);
    await expect(documented.json()).resolves.toMatchObject({
      code: "DEPENDENCY_UNAVAILABLE",
      requestId: "req-1",
    });

    for (const payload of [
      { code: "UNKNOWN_CODE", message: "bad", requestId: "req-2", fieldErrors: [] },
      { code: "VERSION_CONFLICT", message: "bad", requestId: "req-3", fieldErrors: [] },
    ]) {
      const invalid = await buildWorkbenchBrowserResponse(
        new Response(JSON.stringify(payload), {
          status: 503,
          headers: { "Content-Type": "application/json" },
        }),
        "source-account-mutation",
        ACCOUNT_ID,
        { sourceMutation: true, requestId: "req-safe" },
      );
      expect(invalid.status).toBe(503);
      await expect(invalid.json()).resolves.toMatchObject({ code: "OUTCOME_UNKNOWN" });
    }
  });
});
