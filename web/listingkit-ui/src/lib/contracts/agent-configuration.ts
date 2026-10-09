import { z } from "zod";
import { knowledgeId } from "../api/knowledge";
export const configVersion = z
  .string()
  .max(19)
  .regex(/^[1-9][0-9]*$/)
  .refine(
    (v) =>
      /^[1-9][0-9]{0,18}$/.test(v) &&
      BigInt(v) <= BigInt("9223372036854775807"),
  );
export const templateRefSchema = z.strictObject({
  templateId: knowledgeId,
  revision: configVersion,
});
const agentId = z.enum(["product.title.agent","product.image.agent"]),
  timestamp = z.string().datetime({ offset: true }),
  platform = z.enum(["shein", "temu", "amazon"]);
export const templateInputSchema = z.strictObject({
  name: z
    .string()
    .trim()
    .min(1)
    .max(120)
    .refine(
      (v) =>
        new TextEncoder().encode(v).length <= 512 &&
        !/[\u0000-\u001f\u007f]/.test(v),
    ),
  targetPlatform: platform,
  defaultKnowledgeBaseId: knowledgeId.nullable().optional(),
});
export const configReceiptSchema = z.strictObject({
  commandId: knowledgeId,
  operation: z.enum([
    "enable",
    "disable",
    "default",
    "create-template",
    "update-template",
    "archive-template",
  ]),
  agentId,
  templateId: knowledgeId.optional(),
  beforeRevision: configVersion.optional(),
  revision: configVersion,
  version: configVersion.optional(),
  noop: z.boolean(),
  committedAt: timestamp,
});
export const templateSchema = z.strictObject({
  templateId: knowledgeId,
  agentId:z.literal("product.title.agent"),
  lifecycle: z.enum(["ACTIVE", "ARCHIVED"]),
  revision: configVersion,
  version: configVersion,
  schemaVersion: z.literal("title-config-v1"),
  name: z.string().min(1).max(512),
  targetPlatform: platform,
  defaultKnowledgeBaseId: knowledgeId.optional(),
  knowledgeAvailability: z.enum(["AVAILABLE", "UNAVAILABLE"]).optional(),
  createdAt: timestamp,
});
const configuration = z.strictObject({
  agentId,
  activation: z.enum(["NOT_ENABLED", "ENABLED", "DISABLED"]),
  revision: z.union([configVersion, z.literal("")]),
  activationEpoch: z.union([configVersion, z.literal("")]),
  defaultTemplate: templateRefSchema.nullable(),
  updatedAt: timestamp,
});
export const catalogEntrySchema = z.strictObject({
  agent: configuration,
  name: z.string().min(1).max(512),
  description: z.string().max(2048),
  definitionVersion: z.string().max(128),
  parameterSchema: z.enum(["title-config-v1","image-config-v1"]),
  canConfigure: z.boolean(),
  canUse: z.boolean(),
  canReadRuns: z.boolean(),
  capabilities: z
    .array(
      z.strictObject({
        id: z.enum([
          "text.generate",
          "product.source-evidence",
          "knowledge.context",
          "image.generate",
          "platform.write",
        ]),
        support: z.enum(["REQUIRED", "OPTIONAL", "NOT_SUPPORTED"]),
        readiness: z.enum([
          "AVAILABLE",
          "NEEDS_CONFIGURATION",
          "REQUIRES_AUTHORIZATION",
          "UNAVAILABLE",
        ]),
        reason: z.string().max(2048),
        observedAt: timestamp,
      }),
    )
    .max(5),
}).refine(entry=>(entry.agent.agentId==="product.image.agent") === (entry.parameterSchema==="image-config-v1"),{message:"智能体与参数类型不一致"});
const page = <T extends z.ZodType>(schema: T, max = 100) =>
  z.strictObject({
    items: z.array(schema).max(max),
    nextCursor: z.string().max(180),
  });
export const catalogPageSchema = page(catalogEntrySchema);
export const templatesPageSchema = page(templateSchema);
export const recentRunsSchema = page(
  z.strictObject({
    runId: knowledgeId,
    operationId: knowledgeId,
    requestKey: knowledgeId,
    productKey: z.string().min(1).max(128),
    targetPlatform: platform,
    phase: z.enum([
      "running",
      "interrupted",
      "human_review_required",
      "stopped",
    ]),
    revision: configVersion,
    startedAt: timestamp,
  }),
  20,
);
export type CatalogEntry = z.infer<typeof catalogEntrySchema>;
export type AgentTemplate = z.infer<typeof templateSchema>;
export type TemplateRef = z.infer<typeof templateRefSchema>;
