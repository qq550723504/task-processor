import type { NavigationTarget, QueueQuery, ReviewTarget } from "@/lib/types/listingkit";

function reviewTargetFromQuery(query?: QueueQuery): ReviewTarget | undefined {
  if (!query) {
    return undefined;
  }

  const platform = query.platform;
  const slot = query.slot;
  const capability = query.preview_capability;
  const sectionKey = query.from_section_key;

  if (!platform && !slot && !capability && !sectionKey) {
    return undefined;
  }

  return {
    platform,
    slot,
    capability,
    section_key: sectionKey,
  };
}

export function deriveWorkspaceTargetFromNavigationTarget(
  target?: NavigationTarget | null,
): ReviewTarget | undefined {
  if (!target) {
    return undefined;
  }

  if (
    target.dispatch_kind === "review_session" ||
    target.dispatch_kind === "session" ||
    target.dispatch_kind === "review_preview" ||
    target.dispatch_kind === "preview" ||
    target.session_query ||
    target.preview_query
  ) {
    return (
      reviewTargetFromQuery(target.session_query) ??
      reviewTargetFromQuery(target.preview_query)
    );
  }

  return undefined;
}
