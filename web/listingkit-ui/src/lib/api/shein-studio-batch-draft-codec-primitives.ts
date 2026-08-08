import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioGenerationJob,
} from "@/lib/types/shein-studio";

type RawCreatedTask = {
  id?: string;
  title?: string;
  designId?: string;
  design_id?: string;
};

function asNonBlankString(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
}

export function deriveStudioBatchDraftName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "未命名批次";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}

export function normalizeStudioHotStyleReferenceImageUrls(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  const result: string[] = [];
  const seen = new Set<string>();
  for (const item of value) {
    if (typeof item !== "string") {
      continue;
    }
    const trimmed = item.trim();
    if (!trimmed || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    result.push(trimmed);
    if (result.length === 1) {
      break;
    }
  }
  return result;
}

export function normalizeStudioBatchCreatedTasks(
  input: unknown,
  fallbackDesignIds?: string[],
  fallbackDesigns?: Array<{ id: string } | undefined>,
): SheinStudioCreatedTask[] {
  if (!Array.isArray(input)) {
    return [];
  }

  return input
    .map((item, index) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const raw = item as RawCreatedTask;
      const id = asNonBlankString(raw.id);
      const title = asNonBlankString(raw.title);
      if (!id || !title) {
        return null;
      }
      return {
        id,
        title,
        designId:
          asNonBlankString(raw.designId ?? raw.design_id) ??
          fallbackDesignIds?.[index] ??
          fallbackDesigns?.[index]?.id ??
          "",
      } satisfies SheinStudioCreatedTask;
    })
    .filter((item): item is SheinStudioCreatedTask => Boolean(item));
}

export function normalizeStudioBatchGenerationJobs(
  input: unknown,
): SheinStudioGenerationJob[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input.reduce<SheinStudioGenerationJob[]>((jobs, item) => {
    if (!item || typeof item !== "object") {
      return jobs;
    }
    const job = item as Record<string, unknown>;
    const jobId = typeof job.job_id === "string" ? job.job_id.trim() : "";
    if (!jobId) {
      return jobs;
    }
    jobs.push({
      jobId,
      targetGroupKey:
        typeof job.target_group_key === "string" ? job.target_group_key : undefined,
      targetGroupLabel:
        typeof job.target_group_label === "string"
          ? job.target_group_label
          : undefined,
      status:
        job.status === "succeeded" || job.status === "failed"
          ? job.status
          : "running",
    } satisfies SheinStudioGenerationJob);
    return jobs;
  }, []);
}

export function normalizeStudioBatchDesignResponse(
  design: Record<string, unknown>,
): SheinStudioGeneratedDesign | null {
  if (!design || typeof design !== "object" || typeof design.id !== "string") {
    return null;
  }
  return {
    id: design.id,
    imageUrl:
      typeof design.image_url === "string"
        ? design.image_url
        : typeof design.imageUrl === "string"
          ? design.imageUrl
          : undefined,
    prompt: typeof design.prompt === "string" ? design.prompt : undefined,
    revisedPrompt:
      typeof design.revised_prompt === "string"
        ? design.revised_prompt
        : typeof design.revisedPrompt === "string"
          ? design.revisedPrompt
          : undefined,
    imageModel:
      typeof design.image_model === "string"
        ? design.image_model
        : typeof design.imageModel === "string"
          ? design.imageModel
          : undefined,
    transparentBackground:
      typeof design.transparent_background === "boolean"
        ? design.transparent_background
        : typeof design.transparentBackground === "boolean"
          ? design.transparentBackground
          : undefined,
    variationIntensity:
      design.variation_intensity === "light" ||
      design.variation_intensity === "medium" ||
      design.variation_intensity === "strong"
        ? design.variation_intensity
        : design.variationIntensity === "light" ||
            design.variationIntensity === "medium" ||
            design.variationIntensity === "strong"
          ? design.variationIntensity
          : undefined,
    reviewNote:
      typeof design.review_note === "string"
        ? design.review_note
        : typeof design.reviewNote === "string"
          ? design.reviewNote
          : undefined,
    role: typeof design.role === "string" ? design.role : undefined,
    roleLabel:
      typeof design.role_label === "string"
        ? design.role_label
        : typeof design.roleLabel === "string"
          ? design.roleLabel
          : undefined,
    targetGroupKey:
      typeof design.target_group_key === "string"
        ? design.target_group_key
        : typeof design.targetGroupKey === "string"
          ? design.targetGroupKey
          : undefined,
    targetGroupLabel:
      typeof design.target_group_label === "string"
        ? design.target_group_label
        : typeof design.targetGroupLabel === "string"
          ? design.targetGroupLabel
          : undefined,
    productImageUrls: Array.isArray(design.product_image_urls)
      ? (design.product_image_urls as string[])
      : Array.isArray(design.productImageUrls)
        ? (design.productImageUrls as string[])
        : undefined,
  } satisfies SheinStudioGeneratedDesign;
}
