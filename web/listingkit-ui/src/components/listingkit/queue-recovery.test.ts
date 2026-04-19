import { derivePrimaryRecoveryDescriptor } from "@/components/listingkit/queue-recovery";

describe("derivePrimaryRecoveryDescriptor", () => {
  it("prefers review_fallback descriptors", () => {
    const result = derivePrimaryRecoveryDescriptor([
      { recovery_hint: "retry_dispatch", recovery_severity: "high" },
      { recovery_hint: "review_fallback", recovery_severity: "medium" },
    ]);

    expect(result?.recovery_hint).toBe("review_fallback");
  });

  it("returns undefined when there are no descriptors", () => {
    expect(derivePrimaryRecoveryDescriptor(undefined)).toBeUndefined();
  });
});
