import { presentRecoveryDescriptor } from "@/components/listingkit/shared/hint-presentation";
import type { PlatformCard, RecoveryDescriptor } from "@/lib/types/listingkit";

 function derivePlatformRecoveryDescriptor(
  card: PlatformCard,
): RecoveryDescriptor | undefined {
  const summary = card.recovery_summary;
  if (!summary) {
    return undefined;
  }

  if (summary.primary_descriptor?.platform === card.platform) {
    return summary.primary_descriptor;
  }

  return summary.recommended_descriptors?.find(
    (descriptor) => descriptor.platform === card.platform,
  );
}

export function derivePlatformRecoveryPresentation(card: PlatformCard) {
  const descriptor = derivePlatformRecoveryDescriptor(card);
  if (descriptor) {
    return {
      descriptor,
      presentation: presentRecoveryDescriptor(descriptor),
    };
  }

  return undefined;
}
