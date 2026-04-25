import {
  presentRecoveryDescriptor,
  presentRetryHint,
} from "@/components/listingkit/shared/hint-presentation";

describe("hint-presentation", () => {
  it("maps retry hints to readable labels", () => {
    expect(presentRetryHint("retry_dispatch")).toEqual({
      label: "Retry generation",
      description: "The generation step can be retried immediately.",
    });
  });

  it("maps recovery descriptors to readable content", () => {
    expect(
      presentRecoveryDescriptor({
        recovery_hint: "review_fallback",
        recovery_cta_kind: "review",
        recovery_severity: "medium",
        recovery_urgency: "now",
      }),
    ).toEqual({
      title: "Use fallback review",
      description:
        "A fallback result is available and should be reviewed before retrying.",
      ctaLabel: "Review fallback",
      metaLabel: "Medium severity / act now",
    });
  });

  it("falls back cleanly for unknown hints", () => {
    expect(presentRetryHint("custom_hint")).toEqual({
      label: "Custom hint",
      description: "Follow the current task guidance before retrying.",
    });
  });
});
