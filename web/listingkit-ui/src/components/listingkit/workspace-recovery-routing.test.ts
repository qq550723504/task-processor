import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace-recovery-routing";

describe("shouldSyncPlatformOnRecovery", () => {
  it("syncs platform for review-session recovery targets", () => {
    expect(
      shouldSyncPlatformOnRecovery({
        recovery_target: {
          dispatch_kind: "review_session",
          session_query: {
            platform: "shein",
            slot: "main",
          },
        },
      }),
    ).toBe(true);
  });

  it("does not sync platform for action recovery targets", () => {
    expect(
      shouldSyncPlatformOnRecovery({
        recovery_target: {
          dispatch_kind: "action",
          action_target: {
            action_key: "retry_section_generation",
          },
        },
      }),
    ).toBe(false);
  });
});
