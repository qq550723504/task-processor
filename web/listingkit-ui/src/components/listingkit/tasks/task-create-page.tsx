"use client";

import { useMemo } from "react";

import {
  loadTaskCreateDraft,
  type TaskCreateDraft,
} from "@/components/listingkit/tasks/task-create-draft";
import { TaskCreateForm } from "@/components/listingkit/tasks/task-create-form";
import { inferTaskDraftFocusFromDraft } from "@/components/listingkit/tasks/task-fixes";
import { useCanonicalProducts } from "@/lib/query/use-canonical-products";

export function TaskCreatePage({
  fromTask,
  focus,
  issues,
  productKey,
}: {
  fromTask?: string;
  focus?: "text" | "imageUrls" | "productUrl";
  issues?: Array<"text" | "imageUrls" | "productUrl">;
  productKey?: string;
}) {
  const products = useCanonicalProducts({ page: 1, page_size: 100 });
  const initialValues = useMemo<Partial<TaskCreateDraft> | undefined>(() => {
    const draft = fromTask ? loadTaskCreateDraft(fromTask) ?? undefined : undefined;
    if (!productKey) {
      return draft;
    }
    return { ...draft, productKey };
  }, [fromTask, productKey]);

  const initialFocus = focus ?? inferTaskDraftFocusFromDraft(initialValues);

  return (
    <TaskCreateForm
      initialValues={initialValues}
      initialFocus={initialFocus}
      fieldIssues={issues}
      catalogProducts={products.data?.items}
      catalogProductsLoading={products.isLoading}
      catalogProductsError={products.isError}
    />
  );
}
