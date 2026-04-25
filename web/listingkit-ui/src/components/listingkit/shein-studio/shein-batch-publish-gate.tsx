"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useQueries, useQueryClient } from "@tanstack/react-query";

import { shouldPollTaskResult } from "@/components/listingkit/tasks/task-status-query";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { getListingKitPreview } from "@/lib/api/preview";
import { submitTask } from "@/lib/api/submit";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import { listingKitKeys } from "@/lib/query/keys";
import type { ListingKitPreview, ListingKitTaskResult } from "@/lib/types/listingkit";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio";

type GateStatus = {
  task: SheinStudioCreatedTask;
  result?: ListingKitTaskResult;
  preview?: ListingKitPreview;
};

type GateFilter = "all" | "draft" | "publish" | "blocked" | "submitted";

function deriveGateState(item: GateStatus) {
  const taskStatus = item.result?.status;
  const readiness = item.preview?.shein?.submit_readiness?.status;
  const submission = item.preview?.shein?.submission;
  const canSaveDraft =
    taskStatus === "completed" &&
    (readiness === "ready" || readiness === "ready_with_warnings");
  const canPublish = taskStatus === "completed" && readiness === "ready";

  return {
    taskStatus: taskStatus ?? "unknown",
    readiness: readiness ?? "unknown",
    canSaveDraft,
    canPublish,
    lastSubmissionStatus: submission?.last_status,
    lastSubmissionAction: submission?.last_action,
    lastSubmissionError: submission?.last_error,
  };
}

export function SheinBatchPublishGate({
  tasks,
}: {
  tasks: SheinStudioCreatedTask[];
}) {
  const client = useQueryClient();
  const [activeFilter, setActiveFilter] = useState<GateFilter>("all");
  const [isSavingDrafts, setIsSavingDrafts] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const resultQueries = useQueries({
    queries: tasks.map((task) => ({
      queryKey: listingKitKeys.taskResult(task.id),
      queryFn: () => getListingKitTaskResult(task.id),
      refetchInterval: (query: { state: { data?: ListingKitTaskResult } }) =>
        shouldPollTaskResult(query.state.data?.status) ? 5000 : false,
      refetchOnWindowFocus: true,
    })),
  });

  const previewQueries = useQueries({
    queries: tasks.map((task) => ({
      queryKey: listingKitKeys.preview(task.id),
      queryFn: () => getListingKitPreview(task.id),
      enabled: resultQueries.some((query) => query.data?.status === "completed"),
      refetchOnWindowFocus: true,
    })),
  });

  const gateStatuses = useMemo(
    () =>
      tasks.map((task, index) => ({
        task,
        result: resultQueries[index]?.data,
        preview: previewQueries[index]?.data,
      })),
    [previewQueries, resultQueries, tasks],
  );

  const draftEligible = gateStatuses.filter((item) => deriveGateState(item).canSaveDraft);
  const publishEligible = gateStatuses.filter((item) => deriveGateState(item).canPublish);
  const visibleStatuses = gateStatuses.filter((item) => {
    const gate = deriveGateState(item);

    switch (activeFilter) {
      case "draft":
        return gate.canSaveDraft;
      case "publish":
        return gate.canPublish;
      case "blocked":
        return gate.taskStatus === "completed" && !gate.canSaveDraft;
      case "submitted":
        return Boolean(gate.lastSubmissionStatus);
      default:
        return true;
    }
  });

  if (tasks.length === 0) {
    return null;
  }

  async function refreshTask(taskId: string) {
    await Promise.all([
      client.invalidateQueries({ queryKey: listingKitKeys.taskResult(taskId) }),
      client.invalidateQueries({ queryKey: listingKitKeys.preview(taskId) }),
    ]);
  }

  async function handleBatchSubmit(action: "save_draft" | "publish") {
    const eligible = action === "publish" ? publishEligible : draftEligible;
    if (eligible.length === 0) {
      setError(
        action === "publish"
          ? "No tasks are ready to publish."
          : "No tasks are eligible for save draft.",
      );
      setMessage("");
      return;
    }

    setError("");
    setMessage("");
    if (action === "publish") {
      setIsPublishing(true);
    } else {
      setIsSavingDrafts(true);
    }

    try {
      for (const item of eligible) {
        await submitTask(item.task.id, {
          platform: "shein",
          action,
        });
        await refreshTask(item.task.id);
      }

      setMessage(
        action === "publish"
          ? `Published ${eligible.length} SHEIN tasks.`
          : `Saved draft for ${eligible.length} SHEIN tasks.`,
      );
    } catch (submitError) {
      setError(
        submitError instanceof Error
          ? submitError.message
          : "Failed to submit SHEIN batch action.",
      );
    } finally {
      setIsPublishing(false);
      setIsSavingDrafts(false);
    }
  }

  return (
    <Card className="border-zinc-300 bg-zinc-50/80 p-5">
      <div className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN publish gate
            </p>
            <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
              Submit only tasks that are actually ready.
            </h2>
            <p className="mt-2 text-sm leading-6 text-zinc-600">
              `Save draft` accepts ready or ready-with-warnings. `Publish` only accepts
              tasks whose SHEIN readiness is fully `ready`.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-xs font-semibold text-zinc-700">
              draft eligible: {draftEligible.length}
            </div>
            <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-xs font-semibold text-zinc-700">
              publish eligible: {publishEligible.length}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-3">
          <Button
            disabled={isSavingDrafts || draftEligible.length === 0}
            onClick={() => handleBatchSubmit("save_draft")}
            tone="secondary"
          >
            {isSavingDrafts ? "Saving drafts..." : "Save draft for eligible"}
          </Button>
          <Button
            disabled={isPublishing || publishEligible.length === 0}
            onClick={() => handleBatchSubmit("publish")}
          >
            {isPublishing ? "Publishing..." : "Publish eligible"}
          </Button>
        </div>

        <div className="flex flex-wrap gap-2">
          {[
            ["all", `All ${gateStatuses.length}`],
            ["draft", `Draft eligible ${draftEligible.length}`],
            ["publish", `Publish eligible ${publishEligible.length}`],
            [
              "blocked",
              `Blocked ${
                gateStatuses.filter((item) => {
                  const gate = deriveGateState(item);
                  return gate.taskStatus === "completed" && !gate.canSaveDraft;
                }).length
              }`,
            ],
            [
              "submitted",
              `Submitted ${
                gateStatuses.filter((item) => deriveGateState(item).lastSubmissionStatus)
                  .length
              }`,
            ],
          ].map(([filter, label]) => (
            <button
              className={`rounded-xl border px-3 py-2 text-xs font-semibold transition ${
                activeFilter === filter
                  ? "border-zinc-950 bg-zinc-950 text-white"
                  : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-100"
              }`}
              key={filter}
              onClick={() => setActiveFilter(filter as GateFilter)}
              type="button"
            >
              {label}
            </button>
          ))}
        </div>

        {error ? (
          <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {error}
          </div>
        ) : null}
        {message ? (
          <div className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {message}
          </div>
        ) : null}

        <div className="grid gap-3">
          {visibleStatuses.map((item) => {
            const gate = deriveGateState(item);
            return (
              <div
                className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-zinc-200 bg-white/80 px-4 py-3"
                key={item.task.id}
              >
                <div className="space-y-1">
                  <div className="text-sm font-semibold text-zinc-950">
                    {item.task.title}
                  </div>
                  <div className="text-xs text-zinc-500">{item.task.id}</div>
                  <div className="flex flex-wrap gap-2 text-xs">
                    <span className="rounded-lg bg-zinc-100 px-2 py-1 font-medium text-zinc-700">
                      task: {gate.taskStatus}
                    </span>
                    <span className="rounded-lg bg-zinc-100 px-2 py-1 font-medium text-zinc-700">
                      readiness: {gate.readiness}
                    </span>
                    {gate.canPublish ? (
                      <span className="rounded-lg bg-emerald-100 px-2 py-1 font-medium text-emerald-700">
                        publish eligible
                      </span>
                    ) : gate.canSaveDraft ? (
                      <span className="rounded-lg bg-amber-100 px-2 py-1 font-medium text-amber-700">
                        draft eligible
                      </span>
                    ) : null}
                    {gate.lastSubmissionStatus ? (
                      <span className="rounded-lg bg-zinc-100 px-2 py-1 font-medium text-zinc-700">
                        last: {gate.lastSubmissionAction ?? "submit"} / {gate.lastSubmissionStatus}
                      </span>
                    ) : null}
                  </div>
                  {gate.lastSubmissionError ? (
                    <div className="text-xs text-rose-600">{gate.lastSubmissionError}</div>
                  ) : null}
                </div>

                <div className="flex flex-wrap gap-2">
                  <Link href={`/listing-kits/${item.task.id}/status`}>
                    <Button tone="ghost">Status</Button>
                  </Link>
                  <Link href={`/listing-kits/${item.task.id}/workspace`}>
                    <Button tone="secondary">Workspace</Button>
                  </Link>
                </div>
              </div>
            );
          })}
          {visibleStatuses.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-zinc-200 bg-white/70 px-4 py-6 text-sm text-zinc-500">
              No tasks match the current filter.
            </div>
          ) : null}
        </div>
      </div>
    </Card>
  );
}
