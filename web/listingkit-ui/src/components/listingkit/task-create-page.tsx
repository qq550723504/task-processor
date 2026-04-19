"use client";

import { useMemo } from "react";

import {
  loadTaskCreateDraft,
  type TaskCreateDraft,
} from "@/components/listingkit/task-create-draft";
import { TaskCreateForm } from "@/components/listingkit/task-create-form";
import { inferTaskDraftFocusFromDraft } from "@/components/listingkit/task-fixes";

export function TaskCreatePage({
  fromTask,
  focus,
  issues,
}: {
  fromTask?: string;
  focus?: "text" | "imageUrls" | "productUrl";
  issues?: Array<"text" | "imageUrls" | "productUrl">;
}) {
  const initialValues = useMemo<Partial<TaskCreateDraft> | undefined>(() => {
    if (!fromTask) {
      return undefined;
    }
    return loadTaskCreateDraft(fromTask) ?? undefined;
  }, [fromTask]);

  const initialFocus = focus ?? inferTaskDraftFocusFromDraft(initialValues);

  return (
    <TaskCreateForm
      initialValues={initialValues}
      initialFocus={initialFocus}
      fieldIssues={issues}
    />
  );
}
