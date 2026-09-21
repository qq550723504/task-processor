import { afterEach, describe, expect, it, vi } from "vitest";

import { getReferralEarnings, getReferralRules } from "./account-referral-economics";

afterEach(() => vi.unstubAllGlobals());

describe("account referral economics client", () => {
  it("reads the authoritative earnings projection from the dedicated endpoint", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json({
      schemaVersion: "referral-earnings-v1",
      referrer: "subject-1",
      currency: "CNY",
      pendingMinor: "2000",
      availableMinor: "10000",
      reservedMinor: "0",
      adjustmentMinor: "-500",
      version: "3",
      updatedAt: "2026-09-13T10:00:00Z",
      source: "referral_earnings_projection",
    }));
    vi.stubGlobal("fetch", fetch);

    await expect(getReferralEarnings("subject-1")).resolves.toMatchObject({ referrer: "subject-1", availableMinor: "10000" });
    expect(fetch).toHaveBeenCalledWith("/api/account/referral-earnings", expect.objectContaining({ headers: expect.any(Headers) }));
  });

  it("reads rules from the backend economics contract", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ schemaVersion: "referral-rules-v1", currency: "CNY", commissionRateBps: 1000, settlementPeriodDays: 14, minimumWithdrawalMinor: "10000", withdrawalReview: "manual", earningsBasis: "canonical_settled_payment_refund_chargeback", source: "referral_economics_contract" })));
    await expect(getReferralRules("subject-1")).resolves.toMatchObject({ commissionRateBps: 1000, minimumWithdrawalMinor: "10000" });
  });
});
