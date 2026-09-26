import { z } from "zod";
import { isAcquisitionUUID } from "./product-acquisition";

const uuid = z.string().refine(isAcquisitionUUID);
const sourceId = z.string().regex(/^catalog-image-[1-9][0-9]{0,15}$/);
const safeImageURL = z.string().url().refine((value) => /^https?:\/\//.test(value));

export const mainImageStartRequestSchema = z.object({ sourceImageId: sourceId }).strict();
export const mainImageApprovalRequestSchema = z.object({ planRevision: z.number().int().positive(), resultDigest: z.string().min(1).max(256), actionId: uuid }).strict();
export const mainImageCandidatesSchema = z.object({
  operationId: uuid,
  candidates: z.array(z.object({ id: sourceId, displayUrl: safeImageURL }).strict()).max(4096),
}).strict();
export const mainImageAcceptedSchema = z.object({ runId: uuid, status: z.literal("accepted") }).strict();
export const mainImageResultSchema = z.object({
  runId: uuid,
  status: z.enum(["planning", "awaiting_plan_approval", "executing", "evaluating", "repairing", "awaiting_final_approval", "blocked", "completed", "failed", "cancelled"]),
  planRevision: z.number().int().positive(), resultDigest: z.string().max(256), approvalAvailable: z.boolean(),
  imageUrl: safeImageURL.optional(), blockCode: z.string().max(128).optional(),
}).strict().superRefine((value, ctx) => {
  if (value.approvalAvailable && (value.status !== "awaiting_final_approval" || !value.resultDigest || !value.imageUrl)) {
    ctx.addIssue({ code: "custom", message: "Approval requires a viewable QA-passed candidate" });
  }
});

export type MainImageCandidate = z.infer<typeof mainImageCandidatesSchema>["candidates"][number];
export type MainImageResult = z.infer<typeof mainImageResultSchema>;
