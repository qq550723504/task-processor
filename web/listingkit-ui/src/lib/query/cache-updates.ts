import type { QueryClient } from "@tanstack/react-query";

import { listingKitKeys } from "@/lib/query/keys";
import {
  mergeQueuePage,
  mergeReviewPreview,
  mergeReviewSession,
} from "@/lib/query/normalize";
import type {
  ActionExecutionResult,
  NavigationDispatchResponse,
  QueuePage,
  QueueQuery,
  ReviewPreviewResponse,
  ReviewSessionResponse,
} from "@/lib/types/listingkit";

export function applyQueueCache(
  client: QueryClient,
  taskId: string,
  query: QueueQuery,
  next?: QueuePage,
) {
  if (!next) return;
  client.setQueryData<QueuePage>(listingKitKeys.queue(taskId, query), (current) =>
    mergeQueuePage(current, next),
  );
}

export function applyReviewSessionCache(
  client: QueryClient,
  taskId: string,
  query: QueueQuery,
  next?: ReviewSessionResponse,
) {
  if (!next) return;
  client.setQueryData<ReviewSessionResponse>(
    listingKitKeys.reviewSession(taskId, query),
    (current) => mergeReviewSession(current, next),
  );
}

export function applyReviewPreviewCache(
  client: QueryClient,
  taskId: string,
  query: QueueQuery,
  next?: ReviewPreviewResponse,
) {
  if (!next) return;
  client.setQueryData<ReviewPreviewResponse>(
    listingKitKeys.reviewPreview(taskId, query),
    (current) => mergeReviewPreview(current, next),
  );
}

export function applyDispatchResultToCache(
  client: QueryClient,
  taskId: string,
  baseQuery: QueueQuery,
  response: NavigationDispatchResponse,
) {
  applyQueueCache(client, taskId, baseQuery, response.queue);
  applyReviewSessionCache(client, taskId, baseQuery, response.review_session);
  applyReviewPreviewCache(client, taskId, baseQuery, response.review_preview);

  if (!response.panel_update) return;
  applyReviewSessionCache(
    client,
    taskId,
    baseQuery,
    response.panel_update.review_session,
  );
  applyReviewPreviewCache(
    client,
    taskId,
    baseQuery,
    response.panel_update.review_preview,
  );
}

export function applyActionResultToCache(
  client: QueryClient,
  taskId: string,
  baseQuery: QueueQuery,
  response: ActionExecutionResult,
) {
  applyQueueCache(client, taskId, baseQuery, response.queue);
  if (!response.review_session) return;
  client.setQueryData<ReviewSessionResponse>(listingKitKeys.reviewSession(taskId, baseQuery), {
    task_id: taskId,
    session: response.review_session,
    patch: response.review_patch,
    resolved_action_summary: response.resolved_action_summary,
    recovery_summary: response.recovery_summary,
    conditional: response.conditional,
    delta_token: response.delta_token,
  });
}
