import type { RecoveryDescriptor } from "@/lib/types/listingkit";

export function shouldSyncPlatformOnRecovery(
  descriptor: Pick<RecoveryDescriptor, "recovery_target">,
) {
  const target = descriptor.recovery_target;
  if (!target) {
    return false;
  }

  return Boolean(
    target.dispatch_kind === "review_session" ||
      target.dispatch_kind === "session" ||
      target.dispatch_kind === "review_preview" ||
      target.dispatch_kind === "preview" ||
      target.session_query ||
      target.preview_query,
  );
}
