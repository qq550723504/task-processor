import type { RecoveryDescriptor } from "@/lib/types/listingkit";

const priority: Record<string, number> = {
  review_fallback: 0,
  retry_dispatch: 1,
  refresh_revision: 2,
  wait_for_generation: 3,
};

export function derivePrimaryRecoveryDescriptor(
  descriptors?: RecoveryDescriptor[],
) {
  if (!descriptors?.length) {
    return undefined;
  }

  return [...descriptors].sort((left, right) => {
    const leftPriority = priority[left.recovery_hint ?? "wait_for_generation"] ?? 9;
    const rightPriority =
      priority[right.recovery_hint ?? "wait_for_generation"] ?? 9;
    return leftPriority - rightPriority;
  })[0];
}
