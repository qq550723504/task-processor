import { z } from "zod";
import { isAcquisitionUUID } from "./product-acquisition";
const id = z.string().refine(isAcquisitionUUID);
const version = z.string().max(19).regex(/^[1-9][0-9]*$/).refine(v => BigInt(v) <= BigInt("9223372036854775807"));
const text = z.string().max(4096);
const diagnostic = z.strictObject({ Code: text, Field: text, Message: text, Metadata: z.record(z.string(), z.string()).nullable() });
const change = z.strictObject({ Field: z.string().max(128), Value: text, EvidenceIDs: z.array(z.string().max(128)).max(32).nullable() });
export const agentEmptyRequestSchema = z.strictObject({});
export const agentTargetPlatformSchema = z.enum(["shein", "temu", "amazon"]);
export const agentStartRequestSchema = z.strictObject({ targetPlatform: agentTargetPlatformSchema });
export const agentResumeRequestSchema = z.strictObject({ revision: version, feedback: z.string().refine(v => new TextEncoder().encode(v).length <= 8192) });
export const agentReviewLinkSchema = z.strictObject({ proposalId: id, requestKey: id, operationId: id });
export const agentResultSchema = z.strictObject({
    runId: id, requestKey: id, operationId: id, productKey: z.string().min(1).max(128), catalogVersion: version, publicationId: z.string().min(1).max(128), targetPlatform: agentTargetPlatformSchema,
    phase: z.enum(["running", "interrupted", "human_review_required", "stopped"]), revision: version, stopReason: z.string().max(128).optional(), humanReviewRequired: z.literal(true),
    candidate: z.strictObject({ Changes: z.array(change).max(32).nullable(), Warnings: z.array(diagnostic).max(32).nullable(), Rejections: z.array(diagnostic).max(32).nullable() }),
    confidence: z.array(z.strictObject({ Field: z.string().max(128), Value: z.number().min(0).max(1), Known: z.boolean() })).max(32).nullable(),
    unresolved: z.array(text).max(64).nullable(),
    steps: z.array(z.strictObject({ step: z.number().int().min(0).max(16), tool: z.string().max(128).optional(), callId: z.string().max(128).optional(), invocationId: z.string().max(128).optional(), auditStatus: z.string().max(64).optional() })).max(24),
    tokens: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER), estimatedCostMicros: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER), currency: z.string().regex(/^[A-Z]{3}$/), usageStatus: z.enum(["observed", "unknown_reserved"]), canSubmitReview: z.boolean(),
}).refine(v => !v.canSubmitReview || (v.phase === "human_review_required" && !v.stopReason && v.candidate.Changes?.length === 1 && v.candidate.Changes[0]?.Field === "title"));
export type ProductAgentResult = z.infer<typeof agentResultSchema>;
export function agentPath(path: string[]): string | null {
    if (path[0] !== "sourcing" || path[1] !== "1688" || path[2] !== "acquisitions" || !isAcquisitionUUID(path[3] ?? "") || path[4] !== "product-agent" || path[5] !== "runs")
        return null;
    if (path.length === 6 || path.length === 7 && isAcquisitionUUID(path[6] ?? "") || path.length === 8 && isAcquisitionUUID(path[6] ?? "") && ["resume", "review"].includes(path[7] ?? ""))
        return path.join("/");
    return null;
}
