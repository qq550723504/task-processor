import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

export function buildSheinStudioStepHref(
  pathname: string,
  searchParams: URLSearchParams | { toString(): string },
  step: SheinStudioStepKey,
) {
  const params = sanitizedNavigationSearchParams(searchParams);
  params.set("step", step);
  return `${pathname}?${params.toString()}`;
}
