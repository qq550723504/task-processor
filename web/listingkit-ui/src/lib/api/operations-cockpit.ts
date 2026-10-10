import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
export type CockpitScope = { userId: string; organizationId: string };
export class CockpitError extends Error { constructor(public code: string, public status = 500) { super(code); } }
export async function cockpitRequest<T>(scope: CockpitScope, path: string, schema: z.ZodType<T>, init: RequestInit = {}): Promise<T> {
 const headers = new Headers(init.headers); headers.set("X-Expected-User-ID", scope.userId); headers.set("X-Expected-Organization-ID", scope.organizationId); headers.set("Accept", "application/json");
 const write = init.method === "POST";
 try {
  const response = await fetch("/api/operations-cockpit/" + path, { ...init, headers, signal: init.signal ?? AbortSignal.timeout(16000), credentials: "same-origin", cache: "no-store", redirect: "error" });
  const body = await readBoundedStrictJSON(response, 1 << 20, init.signal ?? undefined);
  if (response.status !== 200) { const error = z.strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) }).safeParse(body); throw new CockpitError(error.success ? error.data.code : write ? "OUTCOME_UNKNOWN" : "DEPENDENCY_UNAVAILABLE", response.status); }
  const parsed = schema.safeParse(body); if (!parsed.success) throw new Error(); return parsed.data;
 } catch (error) { if (error instanceof CockpitError) throw error; throw new CockpitError(write ? "OUTCOME_UNKNOWN" : "DEPENDENCY_UNAVAILABLE"); }
}
export function cockpitErrorText(error: unknown): string {
 const code = error instanceof CockpitError ? error.code : "";
 return ({ FORBIDDEN: "当前成员、模块或店铺权限已不可用，请重新确认权限。", REVISION_MISMATCH: "记录已有新版本。请刷新并核对后重新保存。", PERIOD_OVERLAP: "录入区间与其他记录重叠，请核对经营明细。", OUTCOME_UNKNOWN: "保存结果尚未确认。保留原操作，重试读取真实回执。", IDEMPOTENCY_CONFLICT: "原操作编号对应的内容不同，请保留原操作并联系管理员。", AMOUNT_OUT_OF_RANGE: "汇总超过可精确显示的范围，请缩小查询。", INVALID_REQUEST: "请检查日期、金额及必填成本；没有发生的成本请明确填写 0。", IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请恢复原身份后确认原操作。", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化，请恢复原企业后确认原操作。" } as Record<string, string>)[code] ?? "当前能力暂不可用，请稍后刷新。";
}
