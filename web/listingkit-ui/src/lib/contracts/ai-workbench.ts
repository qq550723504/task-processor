import { z } from "zod";
import { isAcquisitionUUID } from "./product-acquisition";

const id = z.string().refine(isAcquisitionUUID);
const timestamp = z.string().datetime({ offset: true });
const scope = z.object({ OrganizationID: z.string(), ActorID: z.string() });
const conversation = z.object({
  ID: id, Scope: scope, Title: z.string(), Favorite: z.boolean(), Archived: z.boolean(),
  MetadataRevision: z.number().int().positive(), NextSequence: z.number().int().positive(),
  CreatedAt: timestamp, UpdatedAt: timestamp,
});
const message = z.object({
  ID: id, ConversationID: id, Sequence: z.number().int().positive(),
  Author: z.enum(["USER", "ASSISTANT"]), Content: z.string(), CreatedAt: timestamp,
});
const proposal = z.object({
  id, digest: z.string().length(64), sourceSequence: z.number().int().positive(),
  goalSummary: z.string(), operationId: z.string().optional(), productKey: z.string().optional(),
  targetPlatform: z.enum(["shein", "temu", "amazon"]).optional(),
  templateId: z.string().optional(), templateRevision: z.string().optional(), knowledgeBaseId: z.string().optional(),
  providerId: z.string().optional(), modelId: z.string().optional(), maximumTokens: z.number().int().optional(),
  maximumCostMicros: z.number().int().optional(), currency: z.string().optional(),
  humanReviewRequired: z.literal(true), detailsAvailable: z.boolean(), titleProfileReady: z.boolean(),
});
const task = z.object({
  id, conversationId: id, proposalId: id, title: z.string(), goalSummary: z.string(), createdAt: timestamp,
  projectionAvailable: z.boolean(), state: z.enum(["RUNNING", "WAITING_CONFIRMATION", "COMPLETED", "ERROR", "PAUSED"]).optional(),
  reason: z.string().optional(), canStart: z.boolean(), canReconcile: z.boolean(), canResume: z.boolean(), canReview: z.boolean(),
  productDetailsAvailable: z.boolean(), operationId: z.string().optional(), productKey: z.string().optional(),
  targetPlatform: z.string().optional(), agentRunId: z.string().optional(), agentPhase: z.string().optional(),
  agentRevision: z.string().optional(), providerId: z.string().optional(), modelId: z.string().optional(),
  tokens: z.number().int().optional(), estimatedCostMicros: z.number().int().optional(), currency: z.string().optional(),
  usageStatus: z.string().optional(), reviewId: z.string().optional(), reviewState: z.string().optional(),
});
export const aiCreateBody = z.strictObject({ favorite: z.boolean().optional() });
export const aiMessageBody = z.strictObject({
  content: z.string().min(1).refine(v => new TextEncoder().encode(v).length <= 8192), operationId: id,
  targetPlatform: z.enum(["shein", "temu", "amazon"]), templateId: id.optional(),
  templateRevision: z.string().regex(/^[1-9][0-9]*$/).optional(), knowledgeBaseId: id.optional(),
});
export const aiMetadataBody = z.strictObject({ title: z.string().max(256).optional(), favorite: z.boolean().optional(), archived: z.boolean().optional() });
export const aiResumeBody = z.strictObject({ revision: z.string().regex(/^[1-9][0-9]*$/), feedback: z.string().max(8192) });

export type AIConversation = z.infer<typeof conversation>;
export type AIProposal = z.infer<typeof proposal>;
export type AITask = z.infer<typeof task>;

export type AIWorkbenchRoute = "conversation-list" | "conversation-create" | "conversation-read" | "conversation-metadata" |
  "message" | "confirm" | "task-list" | "task-read" | "task-start" | "task-resume" | "task-review";

export function aiWorkbenchPath(method: string, path: string[]): AIWorkbenchRoute | null {
  if (path[0] === "chat" && path[1] === "conversations") {
    if (path.length === 2) return method === "GET" ? "conversation-list" : method === "POST" ? "conversation-create" : null;
    if (!isAcquisitionUUID(path[2] ?? "")) return null;
    if (path.length === 3) return method === "GET" ? "conversation-read" : method === "PATCH" ? "conversation-metadata" : null;
    if (path.length === 4 && path[3] === "messages" && method === "POST") return "message";
    if (path.length === 6 && path[3] === "proposals" && isAcquisitionUUID(path[4] ?? "") && path[5] === "confirm" && method === "POST") return "confirm";
  }
  if (path[0] === "tasks") {
    if (path.length === 1 && method === "GET") return "task-list";
    if (!isAcquisitionUUID(path[1] ?? "")) return null;
    if (path.length === 2 && method === "GET") return "task-read";
    if (path.length === 3 && method === "POST") {
      if (path[2] === "start") return "task-start";
      if (path[2] === "resume") return "task-resume";
      if (path[2] === "review") return "task-review";
    }
  }
  return null;
}

export const aiResponseSchemas = {
    "conversation-list": z.object({ conversations: z.array(conversation), next: z.string() }),
    "conversation-create": z.object({ conversation, replay: z.boolean() }),
    "conversation-read": z.object({ conversation, messages: z.array(message), proposals: z.array(proposal), before: z.string() }),
    "conversation-metadata": z.object({ conversation }),
    message: z.object({ state: z.enum(["READY_TO_DISPATCH", "COMPLETE", "FAILED_BEFORE_DISPATCH", "PLANNER_INVALID_OUTPUT", "PLANNER_UNKNOWN"]),
      userMessageId: id, sourceSequence: z.number().int().positive(), assistantMessageId: z.string(), proposalId: z.string(), proposal: proposal.optional() }),
    confirm: z.object({ task, replay: z.boolean() }),
    "task-list": z.object({ tasks: z.array(task), next: z.string() }),
    "task-read": z.object({ task }),
    "task-start": z.object({ task }),
    "task-resume": z.object({ task }),
    "task-review": z.object({ task }),
} as const;
export function parseAIWorkbenchResponse(route: AIWorkbenchRoute, status: number, value: unknown) {
  if (!(status === 200 || route === "message" && status === 202)) return null;
  const checked = aiResponseSchemas[route].safeParse(value);
  return checked.success ? checked.data : null;
}
