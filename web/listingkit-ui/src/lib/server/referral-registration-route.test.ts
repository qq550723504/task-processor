import { mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

import {
  POST as start,
  GET as rejectedGET,
} from "@/app/api/referral-registration/route";
import { POST as resume } from "@/app/api/referral-registration/resume/route";
import { isReferralRegistrationAvailable } from "./referral-registration-route";

const roots: string[] = [];
const credential = "a".repeat(64);

async function configuredCredential() {
  const root = await mkdtemp(path.join(os.tmpdir(), "referral-bff-"));
  roots.push(root);
  const file = path.join(root, "service-credential.txt");
  await writeFile(file, `${credential}\n`, { mode: 0o600 });
  vi.stubEnv("LISTINGKIT_REFERRAL_SERVICE_CREDENTIAL_FILE", file);
}

function request(
  route: "start" | "resume" = "start",
  headers: Record<string, string> = {},
  body?: string,
) {
  const pathname = route === "start" ? "referral-registration" : "referral-registration/resume";
  return new NextRequest(`https://app.test/api/${pathname}`, {
    method: "POST",
    headers: {
      Origin: "https://app.test",
      "Sec-Fetch-Site": "same-origin",
      "Content-Type": "application/json",
      "X-ListingKit-Client-IP": "203.0.113.9",
      ...(route === "start" ? { "Idempotency-Key": "b".repeat(64) } : {}),
      ...headers,
    },
    body:
      body ??
      (route === "start"
        ? JSON.stringify({ code: "ABCD1234", email: "new@example.test", givenName: "新", familyName: "用户" })
        : JSON.stringify({ intentID: "intent-1", resumeSecret: "c".repeat(64) })),
  });
}

beforeEach(async () => {
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://app.test");
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:8085/api/v1");
  await configuredCredential();
});

afterEach(async () => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  await Promise.all(roots.splice(0).map((root) => rm(root, { recursive: true, force: true })));
});

describe("public referral registration BFF", () => {
  it("reports invitation availability from the same private configuration", async () => {
    expect(await isReferralRegistrationAvailable()).toBe(true);
    vi.stubEnv("LISTINGKIT_REFERRAL_SERVICE_CREDENTIAL_FILE", "");
    expect(await isReferralRegistrationAvailable()).toBe(false);
  });
  it("forwards only canonical input plus the server credential and trusted proxy IP", async () => {
    const fetch = vi.fn().mockResolvedValue(
      Response.json({
        intentID: "intent-1",
        resumeSecret: "c".repeat(64),
        createExpiresAt: "2026-09-13T10:15:00Z",
        completionExpiresAt: "2026-09-14T10:00:00Z",
      }),
    );
    vi.stubGlobal("fetch", fetch);

    const response = await start(
      request("start", {
        Cookie: "private=session",
        "X-Forwarded-For": "198.51.100.7",
      }),
    );

    expect(response.status).toBe(200);
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("http://127.0.0.1:8085/api/v1/referral-registration/intents");
    expect(Object.fromEntries(init.headers)).toEqual({
      accept: "application/json",
      "content-type": "application/json",
      "idempotency-key": "b".repeat(64),
      "x-referral-client-ip": "203.0.113.9",
      "x-referral-service-credential": credential,
    });
    expect(init.body).toBe('{"code":"ABCD1234","email":"new@example.test","givenName":"新","familyName":"用户"}');
    expect(init.redirect).toBe("manual");
    expect(init.cache).toBe("no-store");
  });

  it.each([
    ["missing trusted proxy assertion", { "X-ListingKit-Client-IP": "" }, 403],
    ["IPv4 loopback proxy assertion", { "X-ListingKit-Client-IP": "127.14.0.1" }, 403],
    ["IPv6 loopback proxy assertion", { "X-ListingKit-Client-IP": "::1" }, 403],
    ["expanded IPv6 loopback proxy assertion", { "X-ListingKit-Client-IP": "0:0:0:0:0:0:0:1" }, 403],
    ["mapped loopback proxy assertion", { "X-ListingKit-Client-IP": "::ffff:127.0.0.1" }, 403],
    ["cross-site request", { Origin: "https://evil.test", "Sec-Fetch-Site": "cross-site" }, 403],
    ["browser service credential", { "X-Referral-Service-Credential": credential }, 400],
    ["browser Go client IP", { "X-Referral-Client-IP": "198.51.100.8" }, 400],
    ["browser authorization", { Authorization: "Bearer user-token" }, 400],
    ["browser organization identity", { "X-Requested-Organization-ID": "attacker-org" }, 400],
  ])("rejects %s before upstream", async (_name, headers, status) => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    expect((await start(request("start", headers))).status).toBe(status);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("rejects ambiguous or oversized browser input", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    expect((await start(request("start", {}, '{"code":"A","code":"B"}'))).status).toBe(400);
    expect((await start(request("start", {}, JSON.stringify({ code: "A", email: "a@b.test", givenName: "A", familyName: "B", subject: "attacker" })))).status).toBe(400);
    expect((await start(request("start", {}, `{"code":"A","email":"${"x".repeat(4097)}"}`))).status).toBe(413);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("uses the resume route without forwarding idempotency or identity input", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json({ status: "created" }));
    vi.stubGlobal("fetch", fetch);
    expect((await resume(request("resume"))).status).toBe(200);
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("http://127.0.0.1:8085/api/v1/referral-registration/resume");
    expect(init.headers.get("Idempotency-Key")).toBeNull();
    expect(init.headers.get("Authorization")).toBeNull();
  });

  it("rejects malformed success and sanitizes raw upstream failures", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(Response.json({ intentID: "intent-1", resumeSecret: "short", createExpiresAt: "later", completionExpiresAt: "later" }))
      .mockResolvedValueOnce(Response.json({ error: "private_provider_failure", body: "secret" }, { status: 500 }));
    vi.stubGlobal("fetch", fetch);
    const malformed = await start(request());
    expect(malformed.status).toBe(502);
    expect(await malformed.text()).not.toContain("short");
    const failed = await start(request());
    expect(failed.status).toBe(502);
    expect(await failed.text()).not.toContain("secret");
  });

  it("fails closed for missing private credential or non-loopback upstream", async () => {
    vi.stubGlobal("fetch", vi.fn());
    vi.stubEnv("LISTINGKIT_REFERRAL_SERVICE_CREDENTIAL_FILE", "");
    expect((await start(request())).status).toBe(503);
    vi.stubEnv("LISTINGKIT_REFERRAL_SERVICE_CREDENTIAL_FILE", path.join(roots[0], "service-credential.txt"));
    vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "https://backend.example.test/api/v1");
    expect((await start(request())).status).toBe(503);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("rejects non-POST methods instead of implicit Next responses", async () => {
    expect(rejectedGET().status).toBe(405);
  });
});
