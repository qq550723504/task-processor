import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const state = vi.hoisted(() => ({ user: "subject-1", token: "user-access-token", blocked: false, authGate: null as Promise<void> | null }));
vi.mock("@/auth", () => ({
  serverAuth: (handler: (request: NextRequest & { auth?: unknown }) => Promise<Response>) =>
    async (request: NextRequest) => {
      if (state.blocked) return new Promise<Response>(() => {});
      if (state.authGate) await state.authGate;
      return handler(Object.assign(request, { auth: { accessToken: state.token, identityVersion: 3, identity: { userId: state.user, tenantId: "org-1" } } }));
    },
}));

import {
  GET as read,
  POST as createCode,
  DELETE as rejectedDELETE,
} from "@/app/api/account/referrals/route";
import { POST as complete } from "@/app/api/account/referrals/complete/route";

function request(path = "referrals", method = "GET", headers: Record<string, string> = {}, signal?: AbortSignal) {
  return new NextRequest(`https://app.test/api/account/${path}`, {
    method,
    headers: {
      "X-Expected-User-ID": "subject-1",
      ...(method === "POST" ? { Origin: "https://app.test", "Sec-Fetch-Site": "same-origin" } : {}),
      ...headers,
    },
    signal,
  });
}

beforeEach(() => {
  state.user = "subject-1";
  state.token = "user-access-token";
  state.blocked = false;
  state.authGate = null;
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://app.test");
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:8085/api/v1");
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe("authenticated account referrals BFF", () => {
  it("reads only the current Auth.js subject and forwards no organization or service credential", async () => {
    const payload = { code: "CODE1234", codeAvailability: "available", count: 2, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    const fetch = vi.fn().mockResolvedValue(Response.json(payload));
    vi.stubGlobal("fetch", fetch);
    expect((await read(request("referrals", "GET", { Authorization: "Bearer attacker", "X-Requested-Organization-ID": "other", "X-Referral-Service-Credential": "attacker" }))).status).toBe(200);
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("http://127.0.0.1:8085/api/v1/account/referrals");
    expect(Object.fromEntries(init.headers)).toEqual({ accept: "application/json", authorization: "Bearer user-access-token" });
    expect(init.body).toBeUndefined();
  });

  it("rejects stale browser identity before sending and ignores organization switching", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    state.user = "subject-2";
    expect((await read(request())).status).toBe(409);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("requires same-origin for the two explicit writes and sends empty bodies", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(Response.json({ code: "CODE1234" }))
      .mockResolvedValueOnce(Response.json({ status: "complete", intentID: "intent-1", boundAt: "2026-09-13T10:00:00Z" }));
    vi.stubGlobal("fetch", fetch);
    expect((await createCode(request("referrals", "POST"))).status).toBe(200);
    expect((await complete(request("referrals/complete", "POST"))).status).toBe(200);
    expect(fetch.mock.calls[0][1].body).toBeUndefined();
    expect(fetch.mock.calls[1][1].body).toBeUndefined();
    expect((await createCode(request("referrals", "POST", { Origin: "https://evil.test", "Sec-Fetch-Site": "cross-site" }))).status).toBe(403);
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("accepts a zero-byte stream representation without accepting a payload", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json({ code: "CODE1234" }));
    vi.stubGlobal("fetch", fetch);
    const emptyStream = new ReadableStream({ start(controller) { controller.close(); } });
    const actual = new NextRequest("https://app.test/api/account/referrals", {
      method: "POST",
      headers: { Origin: "https://app.test", "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": "subject-1", "Content-Length": "0" },
      body: emptyStream,
      duplex: "half",
    });
    expect((await createCode(actual)).status).toBe(200);
    expect(fetch).toHaveBeenCalledOnce();
  });

  it("keeps authentication within the one request deadline", async () => {
    vi.useFakeTimers();
    state.blocked = true;
    const pending = read(request());
    await vi.advanceTimersByTimeAsync(15001);
    expect((await pending).status).toBe(504);
  });

  it("never dispatches after authentication resolves beyond the total deadline", async () => {
    vi.useFakeTimers();
    let release!: () => void;
    state.authGate = new Promise<void>((resolve) => { release = resolve; });
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    const pending = createCode(request("referrals", "POST"));
    await vi.advanceTimersByTimeAsync(15001);
    expect((await pending).status).toBe(504);
    release();
    await vi.runAllTimersAsync();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("cancels a hanging empty-body read when the request is aborted", async () => {
    const abort = new AbortController();
    let cancelled = false;
    let reading!: () => void;
    const started = new Promise<void>((resolve) => { reading = resolve; });
    const body = new ReadableStream({ pull() { reading(); }, cancel() { cancelled = true; } });
    const actual = new NextRequest("https://app.test/api/account/referrals", {
      method: "POST",
      headers: { Origin: "https://app.test", "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": "subject-1", "Content-Length": "0" },
      body,
      duplex: "half",
      signal: abort.signal,
    });
    const pending = createCode(actual);
    await started;
    abort.abort();
    expect((await pending).status).toBe(504);
    expect(cancelled).toBe(true);
  });

  it("does not accept query, body, or actor selection", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    expect((await read(request("referrals?subject=other"))).status).toBe(400);
    expect((await createCode(new NextRequest("https://app.test/api/account/referrals", { method: "POST", headers: { Origin: "https://app.test", "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": "subject-1", "Content-Type": "application/json" }, body: "{}" }))).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("fails closed for a non-loopback service origin", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "https://backend.example.test/api/v1");
    expect((await read(request())).status).toBe(503);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("rejects unimplemented methods", () => {
    expect(rejectedDELETE().status).toBe(405);
  });
});
