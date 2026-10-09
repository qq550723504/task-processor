import { z } from "zod";
export const toolVersion = z
  .string()
  .regex(/^[1-9][0-9]{0,18}$/)
  .refine((v) => BigInt(v) <= BigInt("9223372036854775807"));
export const toolID = z.string().regex(/^[a-z][a-z-]{0,79}$/);
const text = (max: number, multiline = false) =>
  z
    .string()
    .refine(
      (v) =>
        [...v].length <= max &&
        !(
          multiline
            ? /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/
            : /[\u0000-\u001f\u007f-\u009f]/
        ).test(v),
    );
export const demandInput = z.strictObject({
  kind: z.enum(["DATA", "CONNECTION", "AUTOMATION", "OUTPUT"]),
  title: text(120).refine((v) => !!v.trim()),
  description: text(4000, true).refine((v) => !!v.trim()),
});
export const toolStage = z.enum([
  "SUBMITTED",
  "EVALUATING",
  "PLAN_CONFIRMED",
  "DEVELOPING",
  "DELIVERED",
  "CLOSED",
]);
export const progressInput = z.strictObject({
  stage: toolStage,
  note: text(2000, true).refine((v) => !!v.trim()),
});
const date = z.iso.datetime({ offset: true });
export const activationSchema = z.strictObject({
  toolId: toolID,
  enabled: z.boolean(),
  revision: toolVersion,
  updatedAt: date,
});
export const toolSchema = z.strictObject({
  id: toolID,
  version: z.literal("0.1.0"),
  name: text(120),
  category: z.enum(["采集", "图片", "环境", "数据", "流程", "其他"]),
  description: text(1000, true),
  status: z.enum(["AVAILABLE", "UNAVAILABLE", "DEVELOPING"]),
  reason: text(1000, true),
  activation: activationSchema.nullable(),
  localCapture: z.boolean(),
  onlineCapture: z.boolean(),
  download: z.boolean(),
});
export const marketSchema = z.strictObject({
  tools: z.array(toolSchema).max(50),
  canManage: z.boolean(),
  canCustomize: z.boolean(),
});
export const requestSchema = demandInput.extend({
  id: z.uuid(),
  organizationId: z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/),
  stage: toolStage,
  revision: toolVersion,
  createdAt: date,
  updatedAt: date,
});
export const requestsSchema = z.strictObject({
  items: z.array(requestSchema.omit({ description: true })).max(50),
  nextCursor: z.union([z.literal(""), z.uuid()]),
});
export const detailSchema = z.strictObject({
  request: requestSchema,
  events: z
    .array(
      z.strictObject({
        revision: toolVersion,
        stage: toolStage,
        note: text(2000, true),
        occurredAt: date,
      }),
    )
    .max(1000),
});
export const toolReceipt = z.strictObject({
  commandId: z.uuid(),
  operation: z.enum(["activation", "create", "progress"]),
  id: z.union([z.literal("product-acquisition"), z.uuid()]),
  revision: toolVersion,
  committedAt: date,
});
export type Tool = z.infer<typeof toolSchema>;
export type ToolRequest = z.infer<typeof requestSchema>;
export type ToolDetail = z.infer<typeof detailSchema>;
export function toolEndpoint(url: URL, method: string) {
  if (!url.pathname.startsWith("/api/tool-market/")) return null;
  let p = url.pathname.slice("/api/tool-market/".length).split("/");
  const admin = p[0] === "admin";
  if (admin) p = p.slice(1);
  let output: z.ZodType,
    input: z.ZodType | undefined,
    operation: "activation" | "create" | "progress" | undefined;
  if (
    !admin &&
    p.length === 1 &&
    ["market", "mine"].includes(p[0]) &&
    method === "GET"
  )
    output = marketSchema;
  else if (!admin && p.length === 1 && p[0] === "plugin" && method === "GET")
    output = z.never();
  else if (
    !admin &&
    p.length === 2 &&
    p[0] === "activations" &&
    p[1] === "product-acquisition" &&
    method === "PUT"
  ) {
    output = toolReceipt;
    input = z.strictObject({ enabled: z.boolean() });
    operation = "activation";
  } else if (p[0] === "requests" && p.length === 1 && method === "GET")
    output = requestsSchema;
  else if (
    !admin &&
    p[0] === "requests" &&
    p.length === 1 &&
    method === "POST"
  ) {
    output = toolReceipt;
    input = demandInput;
    operation = "create";
  } else if (
    p[0] === "requests" &&
    z.uuid().safeParse(p[1]).success &&
    p.length === 2 &&
    method === "GET"
  )
    output = detailSchema;
  else if (
    admin &&
    p[0] === "requests" &&
    z.uuid().safeParse(p[1]).success &&
    p[2] === "progress" &&
    p.length === 3 &&
    method === "POST"
  ) {
    output = toolReceipt;
    input = progressInput;
    operation = "progress";
  } else return null;
  if (url.search.length > 512 || (url.search && output !== requestsSchema))
    return null;
  for (const [k, v] of url.searchParams) {
    if (url.searchParams.getAll(k).length !== 1) return null;
    if (k === "cursor") {
      if (!z.uuid().safeParse(v).success) return null;
    } else if (k === "pageSize") {
      if (!/^[1-9][0-9]*$/.test(v) || Number(v) > 50) return null;
    } else return null;
  }
  return {
    path: p.join("/"),
    admin,
    output,
    input,
    operation,
    download: p[0] === "plugin",
  };
}
