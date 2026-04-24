import {
  deriveRecoveryNavigationTarget,
  pickWorkspaceResolvedActionSummary,
} from "@/components/listingkit/workspace-action-routing";

describe("pickWorkspaceResolvedActionSummary", () => {
  it("prefers actionable overview summaries over passive session review summaries", () => {
    expect(
      pickWorkspaceResolvedActionSummary(
        {
          source_kind: "review_target",
          title: "Review Previews",
          navigation_target: {
            dispatch_kind: "session",
          },
        },
        {
          source_kind: "generation_action",
          title: "Upgrade Fallback Assets",
          action_key: "upgrade_fallback_assets",
          action_target: {
            action_key: "upgrade_fallback_assets",
          },
        },
      ),
    ).toEqual(
      expect.objectContaining({
        title: "Upgrade Fallback Assets",
      }),
    );
  });
});

describe("deriveRecoveryNavigationTarget", () => {
  it("returns the explicit recovery target when available", () => {
    expect(
      deriveRecoveryNavigationTarget({
        recovery_target: {
          dispatch_kind: "session",
          session_query: {
            platform: "shein",
            slot: "main",
          },
        },
      }),
    ).toEqual(
      expect.objectContaining({
        dispatch_kind: "session",
      }),
    );
  });

  it("builds a queue navigation target from descriptor dispatch steps", () => {
    expect(
      deriveRecoveryNavigationTarget({
        descriptor: {
          resource_kind: "generation_queue",
          dispatch_plan: {
            steps: [
              {
                kind: "queue",
                query: {
                  platform: "shein",
                  slot: "gallery",
                },
              },
            ],
          },
        },
      }),
    ).toEqual(
      expect.objectContaining({
        dispatch_kind: "queue",
        queue_query: expect.objectContaining({
          platform: "shein",
          slot: "gallery",
        }),
      }),
    );
  });
});
