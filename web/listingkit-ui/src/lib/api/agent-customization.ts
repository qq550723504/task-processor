import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
export const customId = z.uuid().refine(v => v === v.toLowerCase() && v !== "00000000-0000-0000-0000-000000000000");
export const customVersion = z.string().regex(/^[1-9][0-9]{0,18}$/).refine(v => BigInt(v) <= BigInt("9223372036854775807"));
export const customStages = ["SUBMITTED", "EVALUATING", "PROPOSED", "DEVELOPING", "DELIVERED"] as const;
const customStage = z.enum(customStages);
export const stageLabels: Record<typeof customStages[number], string> = { SUBMITTED: "提交需求", EVALUATING: "需求评估", PROPOSED: "方案确认", DEVELOPING: "开发测试", DELIVERED: "交付使用" };
const text = (max: number, multi = false) => z.string().min(1).max(max).refine(v => v.trim() === v && !new RegExp(multi ? "[\\x00-\\x08\\x0b\\x0c\\x0e-\\x1f\\x7f]" : "[\\x00-\\x1f\\x7f]").test(v));
const customInputFields = z.strictObject({ name: text(120), scenario: text(240), direction: z.enum(["PRODUCT_SUPPLY", "STORE_OPERATIONS", "DATA_ANALYSIS", "OTHER"]), description: text(10000, true), contactName: text(80), contactMethod: text(160), consent: z.literal(true) });
export const customInput = customInputFields.extend({ files: z.array(z.strictObject({ name: text(200).refine(v => !/[\\/]/.test(v)), data: z.string().min(4).max(2796204).regex(/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/) })).max(3).optional() });
export const customUpdate = z.strictObject({ stage: customStage, note: text(5000, true), proposal: text(5000, true).optional(), offlineConfirmation: text(5000, true).optional(), deliverQualityAgent: z.boolean().optional() });
const attachment = z.strictObject({ id: customId, name: text(200), contentType: z.enum(["application/pdf", "image/png", "image/jpeg", "text/plain; charset=utf-8"]), size: z.number().int().min(1).max(2 * 1024 * 1024) });
const at = z.iso.datetime({ offset: true });
const customRequest = z.strictObject({ id: customId, organizationId: text(128), createdBy: text(256), input: customInputFields, stage: customStage, version: customVersion, proposal: z.string().max(5000), offlineConfirmation: z.string().max(5000), consentVersion: z.literal("agent-customization-contact-v1"), attachments: z.array(attachment).max(3), createdAt: at, updatedAt: at, deliveryId: customId.optional() });
const customEvent = z.strictObject({ version: customVersion, actorId: text(256), stage: customStage, note: text(5000, true), proposal: text(5000, true).optional(), offlineConfirmation: text(5000, true).optional(), deliverQualityAgent:z.boolean().optional(), at });
export const customDetail = z.strictObject({ request: customRequest, events: z.array(customEvent).max(50), nextEventVersion: z.union([customVersion, z.literal("")]) });
export const customPage = z.strictObject({ items: z.array(customRequest).max(20), nextCursor: z.union([customId, z.literal("")]) });
export const customReceipt = z.strictObject({ requestId: customId, key: customId, version: customVersion, stage: customStage, at });
export type CustomRequest = z.infer<typeof customRequest>;
export type CustomDetail = z.infer<typeof customDetail>;
export type CustomScope = {
    userId: string;
    organizationId: string;
    admin?: boolean;
};
export class CustomizationError extends Error {
    constructor(public code: string, public status = 500) { super(code); }
}
export async function customizationRequest<T>(scope: CustomScope, path: string, schema: z.ZodType<T>, init: RequestInit = {}, resource: "requests" | "agents" = "requests"): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("X-Expected-User-ID", scope.userId);
    headers.set("X-Expected-Organization-ID", scope.organizationId);
    headers.set("Accept", "application/json");
    const write = init.method && init.method !== "GET";
    try {
        const response = await fetch(`/api/workbench/${scope.admin ? "admin/" : ""}agent-customization/${resource}${path}`, { ...init, headers, cache: "no-store", credentials: "same-origin", redirect: "error" });
        const raw = await readBoundedStrictJSON(response, 4 * 1024 * 1024, init.signal ?? undefined);
        if (!response.ok) {
            const failure = z.object({ code: z.string() }).safeParse(raw);
            throw new CustomizationError(failure.success ? failure.data.code : write ? "OUTCOME_UNKNOWN" : "CUSTOMIZATION_UNAVAILABLE", response.status);
        }
        const value = schema.safeParse(raw);
        if (!value.success)
            throw new Error("invalid upstream");
        return value.data;
    }
    catch (e) {
        if (e instanceof CustomizationError)
            throw e;
        throw new CustomizationError(write ? "OUTCOME_UNKNOWN" : "CUSTOMIZATION_UNAVAILABLE");
    }
}
