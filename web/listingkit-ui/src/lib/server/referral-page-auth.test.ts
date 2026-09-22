import { afterEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({ blocked: false }));
vi.mock("@/auth", () => ({
  serverAuth: () => state.blocked ? new Promise(() => {}) : Promise.resolve({ accessToken: "token", identity: { userId: "subject-1" } }),
}));
vi.mock("next/navigation", () => ({ redirect: (path: string) => { throw new Error(`REDIRECT:${path}`); } }));

import { requireReferralUserId } from "./referral-page-auth";

afterEach(() => {
  vi.useRealTimers();
  state.blocked = false;
});

describe("referral page authentication", () => {
  it("bounds server authentication before rendering a referral page", async () => {
    vi.useFakeTimers();
    state.blocked = true;
    const pending = requireReferralUserId("/workbench/account/referrals/earnings");
    const assertion = expect(pending).rejects.toThrow("REDIRECT:/login?returnTo=%2Fworkbench%2Faccount%2Freferrals%2Fearnings");
    await vi.advanceTimersByTimeAsync(15001);
    await assertion;
  });
});
