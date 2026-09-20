import { afterEach, describe, expect, it, vi } from "vitest";

import {
  ReferralRequestError,
  completeReferralRegistration,
  createAccountReferralCode,
  getAccountReferrals,
  startReferralRegistration,
} from "./referrals";

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("referral browser API", () => {
  it("sends only bounded public registration fields and a stable supplied key", async () => {
    const payload = { intentID: "intent-1", resumeSecret: "a".repeat(64), createExpiresAt: "2026-09-13T10:15:00Z", completionExpiresAt: "2026-09-14T10:00:00Z" };
    const fetch = vi.fn().mockResolvedValue(Response.json(payload));
    vi.stubGlobal("fetch", fetch);
    await expect(startReferralRegistration({ code: "CODE1234", email: "new@example.test", givenName: "新", familyName: "用户" }, "b".repeat(64))).resolves.toEqual(payload);
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("/api/referral-registration");
    expect(init.headers).toEqual({ "Content-Type": "application/json", "Idempotency-Key": "b".repeat(64) });
    expect(init.credentials).toBe("same-origin");
    expect(init.redirect).toBe("error");
  });

  it("binds personal reads and writes to the expected subject guard", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    const fetch = vi.fn()
      .mockResolvedValueOnce(Response.json(projection))
      .mockResolvedValueOnce(Response.json({ code: "CODE1234" }))
      .mockResolvedValueOnce(Response.json({ status: "complete", intentID: "intent-1", boundAt: "2026-09-13T10:00:00Z" }));
    vi.stubGlobal("fetch", fetch);
    await expect(getAccountReferrals("subject-1")).resolves.toEqual(projection);
    await expect(createAccountReferralCode("subject-1")).resolves.toEqual({ code: "CODE1234" });
    await expect(completeReferralRegistration("subject-1")).resolves.toMatchObject({ status: "complete" });
    for (const [, init] of fetch.mock.calls) expect(init.headers["X-Expected-User-ID"]).toBe("subject-1");
  });

  it("distinguishes a true empty projection from unavailable earnings", async () => {
    const projection = { code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(projection)));
    await expect(getAccountReferrals("subject-1")).resolves.toEqual(projection);
  });

  it("rejects malformed success and preserves safe UNKNOWN outcomes", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(Response.json({ code: "CODE1234", codeAvailability: "available", count: -1, generatedAt: "later", earnings: { availability: "unavailable", amount: null } })).mockResolvedValueOnce(Response.json({ code: "referral_outcome_unknown", message: "safe", requestId: "", fieldErrors: [] }, { status: 503 })));
    await expect(getAccountReferrals("subject-1")).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
    await expect(completeReferralRegistration("subject-1")).rejects.toEqual(expect.objectContaining<Partial<ReferralRequestError>>({ status: 503, code: "referral_outcome_unknown" }));
  });

  it("aborts at the single deadline", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", vi.fn((_url, init) => new Promise((_resolve, reject) => init.signal.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")), { once: true }))));
    const pending = getAccountReferrals("subject-1");
    const assertion = expect(pending).rejects.toMatchObject({ status: 504, code: "DEADLINE_EXCEEDED" });
    await vi.advanceTimersByTimeAsync(15001);
    await assertion;
  });
});
