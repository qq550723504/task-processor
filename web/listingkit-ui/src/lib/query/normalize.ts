import type {
  QueuePage,
  ReviewPreviewResponse,
  ReviewSessionResponse,
} from "@/lib/types/listingkit";

export function normalizeConditionalResponse<T extends { not_modified?: boolean }>(
  response: T,
  previous?: T,
) {
  if (response.not_modified && previous) {
    const previousConditional =
      "conditional" in previous && previous.conditional
        ? previous.conditional
        : undefined;
    const responseConditional =
      "conditional" in response && response.conditional
        ? response.conditional
        : undefined;

    return {
      ...previous,
      conditional: {
        ...(previousConditional ?? {}),
        ...(responseConditional ?? {}),
        not_modified: true,
      },
      not_modified: true,
    } as T;
  }

  return response;
}

export function mergeQueuePage(previous?: QueuePage, next?: QueuePage) {
  if (!previous) return next;
  if (!next) return previous;
  return {
    ...previous,
    ...next,
    summary: next.summary ?? previous.summary,
    items: next.items ?? previous.items,
  };
}

export function mergeReviewSession(
  previous?: ReviewSessionResponse,
  next?: ReviewSessionResponse,
) {
  if (!previous) return next;
  if (!next) return previous;
  return {
    ...previous,
    ...next,
    session: next.session ?? previous.session,
    patch: next.patch ?? previous.patch,
  };
}

export function mergeReviewPreview(
  previous?: ReviewPreviewResponse,
  next?: ReviewPreviewResponse,
) {
  if (!previous) return next;
  if (!next) return previous;
  return {
    ...previous,
    ...next,
    preview: next.preview ?? previous.preview,
    toolbar: next.toolbar ?? previous.toolbar,
  };
}
