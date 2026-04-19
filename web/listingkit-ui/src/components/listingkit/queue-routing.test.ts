import { deriveWorkspaceTargetFromNavigationTarget } from "@/components/listingkit/queue-routing";

describe("deriveWorkspaceTargetFromNavigationTarget", () => {
  it("routes review-session targets into workspace focus", () => {
    expect(
      deriveWorkspaceTargetFromNavigationTarget({
        dispatch_kind: "review_session",
        session_query: {
          platform: "shein",
          slot: "main",
          preview_capability: "detail_preview",
          from_section_key: "detail_preview-main",
        },
      }),
    ).toEqual({
      platform: "shein",
      slot: "main",
      capability: "detail_preview",
      section_key: "detail_preview-main",
    });
  });

  it("ignores queue-only targets", () => {
    expect(
      deriveWorkspaceTargetFromNavigationTarget({
        dispatch_kind: "queue",
        queue_query: {
          platform: "temu",
          slot: "gallery",
        },
      }),
    ).toBeUndefined();
  });
});
