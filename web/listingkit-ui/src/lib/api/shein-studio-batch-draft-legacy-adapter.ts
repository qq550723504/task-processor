import {
  normalizeStudioBatchCreatedTasks,
  normalizeStudioBatchDesignResponse,
  normalizeStudioBatchGenerationJobs,
} from "@/lib/api/shein-studio-batch-draft-codec-primitives";
import type { SheinStudioLegacyCompatibilitySnapshot } from "@/lib/types/shein-studio";

export type StudioBatchDraftLegacySnapshotOverrides = Partial<
  SheinStudioLegacyCompatibilitySnapshot
>;

const LEGACY_SNAPSHOT_KEYS = [
  "designs",
  "selectedIds",
  "createdTasks",
  "generationJobs",
  "generationError",
  "generationJobId",
] as const;

function isSemanticallyEmpty(snapshot: SheinStudioLegacyCompatibilitySnapshot) {
  return (
    (snapshot.designs?.length ?? 0) === 0 &&
    (snapshot.selectedIds?.length ?? 0) === 0 &&
    (snapshot.createdTasks?.length ?? 0) === 0 &&
    (snapshot.generationJobs?.length ?? 0) === 0 &&
    !snapshot.generationError &&
    !snapshot.generationJobId
  );
}

export function encodeStudioBatchDraftLegacySnapshot(
  snapshot: SheinStudioLegacyCompatibilitySnapshot | undefined,
) {
  if (!snapshot || isSemanticallyEmpty(snapshot)) {
    return undefined;
  }

  return {
    approved_design_ids: snapshot.selectedIds,
    created_tasks: snapshot.createdTasks,
    generation_jobs: snapshot.generationJobs?.map((job) => ({
      job_id: job.jobId,
      target_group_key: job.targetGroupKey,
      target_group_label: job.targetGroupLabel,
      status: job.status,
    })),
    generation_error: snapshot.generationError,
    generation_job_id: snapshot.generationJobId,
    designs: (snapshot.designs ?? []).map((design) => ({
      id: design.id,
      image_url: design.imageUrl ?? design.dataUrl,
      prompt: design.prompt,
      revised_prompt: design.revisedPrompt,
      image_model: design.imageModel,
      transparent_background: design.transparentBackground,
      transparent_background_mode: design.transparentBackgroundMode,
      variation_intensity: design.variationIntensity,
      review_note: design.reviewNote,
      role: design.role,
      role_label: design.roleLabel,
      target_group_key: design.targetGroupKey,
      target_group_label: design.targetGroupLabel,
      product_image_urls: design.productImageUrls,
    })),
  };
}

export function decodeStudioBatchDraftLegacySnapshot(
  value: Record<string, unknown> | undefined,
): SheinStudioLegacyCompatibilitySnapshot | undefined {
  if (!value) {
    return undefined;
  }

  const selectedIds = Array.isArray(value.approved_design_ids)
    ? (value.approved_design_ids as unknown[]).filter(
        (item): item is string => typeof item === "string",
      )
    : [];
  const designs = Array.isArray(value.designs)
    ? (value.designs as Array<Record<string, unknown>>)
        .map((design) => normalizeStudioBatchDesignResponse(design))
        .filter((design): design is NonNullable<typeof design> => Boolean(design))
    : [];
  const snapshot = {
    designs,
    selectedIds,
    createdTasks: normalizeStudioBatchCreatedTasks(
      Array.isArray(value.created_tasks) ? value.created_tasks : undefined,
      selectedIds,
      designs,
    ),
    generationJobs: normalizeStudioBatchGenerationJobs(
      Array.isArray(value.generation_jobs) ? value.generation_jobs : undefined,
    ),
    generationError:
      typeof value.generation_error === "string" ? value.generation_error : undefined,
    generationJobId:
      typeof value.generation_job_id === "string" ? value.generation_job_id : undefined,
  } satisfies SheinStudioLegacyCompatibilitySnapshot;

  return isSemanticallyEmpty(snapshot) ? undefined : snapshot;
}

export function mergeStudioBatchDraftLegacySnapshot(
  snapshot: SheinStudioLegacyCompatibilitySnapshot | undefined,
  overrides: StudioBatchDraftLegacySnapshotOverrides,
): SheinStudioLegacyCompatibilitySnapshot | undefined {
  const merged: SheinStudioLegacyCompatibilitySnapshot = { ...(snapshot ?? {}) };
  for (const key of LEGACY_SNAPSHOT_KEYS) {
    if (Object.prototype.hasOwnProperty.call(overrides, key)) {
      Object.assign(merged, { [key]: overrides[key] });
    }
  }
  return isSemanticallyEmpty(merged) ? undefined : merged;
}
