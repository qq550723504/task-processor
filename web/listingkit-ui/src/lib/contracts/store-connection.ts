import { z } from "zod";
export const officialConnectionViewSchema = z
	.object({
		appId:z.string().min(1).max(128).optional(),
		appRevision:z.string().min(1).max(200).optional(),
		attemptId: z.union([z.uuid(), z.literal("")]),
    state: z.enum([
      "disconnected",
      "awaiting_consent",
      "exchange_dispatched",
      "credential_received",
      "verified",
      "failed",
    ]),
    connectionStatus: z.enum([
      "connected",
      "disconnected",
      "expired",
      "unavailable",
    ]),
    version: z.number().int().min(0).max(Number.MAX_SAFE_INTEGER),
    observedAt: z.string().datetime({ precision: null }).nullable(),
  })
  .strict();
export const officialConnectionBeginSchema = z
  .object({
    attemptId: z.uuid(),
    authorizationUrl: z
      .string()
      .max(4096)
      .refine((raw) => {
        try {
          const url = new URL(raw);
          return (
            url.origin === "https://openapi-sem.sheincorp.com" &&
            !url.username &&
            !url.password &&
            url.pathname === "/" &&
            !url.search &&
            url.hash.startsWith("#/empower?") &&
            new URLSearchParams(url.hash.slice(10)).get("state")?.length === 43
          );
        } catch {
          return false;
        }
      }),
    expiresAt: z.string().datetime({ precision: null }),
  })
  .strict();
export const officialConnectionCompleteSchema = z
  .object({
    attemptId: z.uuid(),
    appId: z.string().min(1).max(200),
    state: z.string().min(1).max(128),
    tempToken: z.string().min(1).max(4096),
  })
  .strict();
export const officialConnectionQuerySchema = z
  .object({ attemptId: z.uuid() })
	.strict();
const officialApplicationChoicesSchema=z.array(z.object({
	appId:z.string().min(1).max(128),revision:z.string().min(1).max(200),
	type:z.enum(["self_operated","semi_managed","fully_managed"]),
}).strict()).max(16).refine(values=>new Set(values.map(v=>v.appId)).size===values.length);
export const officialConnectionStartSchema=z.object({appId:z.string().min(1).max(128)}).strict();
export const officialApplicationListSchema=z.object({applications:officialApplicationChoicesSchema}).strict();
export type OfficialConnectionCallback = z.infer<
  typeof officialConnectionCompleteSchema
>;
