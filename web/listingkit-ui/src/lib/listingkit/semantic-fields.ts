import type {
  ListingKitTaskResultData,
  SDSSyncSummary,
  SheinPreviewPayload,
  SheinPreviewProductPayload,
  SheinRequestDraftPreview,
  SheinSubmissionReport,
} from "@/lib/types/listingkit";

export function getTaskSDSDesignResult(
  result?: ListingKitTaskResultData | null,
): SDSSyncSummary | undefined {
  return result?.sds_design_result ?? result?.sds_sync;
}

export function getSheinDraftPayload(
  shein?: SheinPreviewPayload | null,
): SheinRequestDraftPreview | undefined {
  return shein?.draft_payload ?? shein?.request_draft;
}

export function getSheinPreviewPayload(
  shein?: SheinPreviewPayload | null,
): SheinPreviewProductPayload | undefined {
  return shein?.preview_payload ?? shein?.preview_product;
}

export function getSheinSubmissionState(
  shein?: Pick<SheinPreviewPayload, "submission" | "submission_state"> | null,
): SheinSubmissionReport | undefined {
  return shein?.submission_state ?? shein?.submission;
}
