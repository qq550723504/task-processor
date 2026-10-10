import { z } from "zod";

export const cockpitID = z.uuid().refine(v => v === v.toLowerCase() && v !== "00000000-0000-0000-0000-000000000000");
const cockpitRevision = z.string().regex(/^[1-9][0-9]{0,18}$/).refine(v => BigInt(v) <= BigInt("9223372036854775807"));
const expected = z.union([z.literal("0"), cockpitRevision]);
const money = z.number().int().min(0).max(1e12);
const safe = z.number().int().min(-Number.MAX_SAFE_INTEGER).max(Number.MAX_SAFE_INTEGER);
const date = z.iso.date();
const stamp = z.iso.datetime({ offset: true });
const cockpitPeriod = z.strictObject({ startDate: date, endDate: date });
const cockpitAmounts = z.strictObject({ revenue: money, refunds: money, procurement: money, logistics: money, platform: money, advertising: money, other: money });
export const cockpitRational = z.strictObject({ numerator: z.string().regex(/^-?(0|[1-9][0-9]{0,39})$/), denominator: z.string().regex(/^[1-9][0-9]{0,39}$/) });
const cockpitTotals = z.strictObject({ revenue: safe, refunds: safe, procurement: safe, logistics: safe, platform: safe, advertising: safe, other: safe, netRevenue: safe, netProfit: safe, margin: cockpitRational.nullable() });
const cockpitStore = z.strictObject({ id: cockpitID, name: z.string().max(400), platform: z.string().max(64), region: z.string().max(100), status: z.string().max(32) });
const cockpitAccess = z.strictObject({ goalsRead: z.boolean(), goalsCreate: z.boolean(), goalsManage: z.boolean(), storesRead: z.boolean(), factsWrite: z.boolean(), alertsRead: z.boolean(), adviceRead: z.boolean() });
export const cockpitCapabilities = z.strictObject({ access: cockpitAccess, today: date, stores: z.array(cockpitStore).max(500) });
const cockpitGoalConfig = z.strictObject({ storeIds: z.array(cockpitID).min(1).max(50).refine(ids => new Set(ids).size === ids.length), frequency: z.enum(["day", "week", "month"]), period: cockpitPeriod, profit: money.positive(), minimumMarginBps: z.number().int().min(0).max(10000).nullable(), normalBps: z.number().int().min(1).max(10000), attentionBps: z.number().int().min(1).max(10000) });
export const cockpitGoal = z.strictObject({ id: cockpitID, creatorId: z.string().min(1).max(200), revision: cockpitRevision, config: cockpitGoalConfig, updatedBy: z.string().min(1).max(200), updatedAt: stamp });
const cockpitEvaluation = z.strictObject({ state: z.enum(["normal", "attention", "abnormal", "not_started", "pending_data"]), through: z.string().max(10), totals: cockpitTotals, expectedProfit: cockpitRational.nullable(), completion: cockpitRational.nullable() });
export const cockpitHead = z.strictObject({ goalId: cockpitID, revision: cockpitRevision, scopeValid: z.boolean(), canReconfigure: z.boolean() });
const evidence = z.strictObject({ id: cockpitID, revision: cockpitRevision, period: cockpitPeriod });
const goalEvidence = z.strictObject({storeId:cockpitID,period:cockpitPeriod,complete:z.boolean(),totals:cockpitTotals,records:z.array(evidence).max(20),excluded:z.array(evidence).max(20),gaps:z.array(cockpitPeriod).max(20),recordCount:z.number().int().min(0).max(366),excludedCount:z.number().int().min(0).max(368),gapCount:z.number().int().min(0).max(366)});
export const cockpitGoals = z.strictObject({ goal: cockpitGoal.nullable(), evaluation: cockpitEvaluation.nullable(), head: cockpitHead.nullable(), goalUnavailable: z.boolean(), capturedAt: stamp,basis:z.array(goalEvidence).max(50) });
export const cockpitRecord = z.strictObject({ id: cockpitID, storeId: cockpitID, revision: cockpitRevision, period: cockpitPeriod, amounts: cockpitAmounts, note: z.string().max(4000), updatedBy: z.string().min(1).max(200), updatedAt: stamp });
const aggregate = z.strictObject({ storeId: cockpitID, period: cockpitPeriod, complete: z.boolean(), totals: cockpitTotals, gaps: z.array(cockpitPeriod).max(366), excluded: z.array(evidence).max(368), records: z.array(evidence).max(366) });
export const cockpitDetail = z.strictObject({ store: cockpitStore, aggregate, growth: cockpitRational.nullable(), capturedAt: stamp });
const cockpitRow = z.strictObject({ store: cockpitStore, complete: z.boolean(), state: z.enum(["loss", "break_even", "recorded_complete", "data_incomplete"]), totals: cockpitTotals, growth: cockpitRational.nullable(), gapCount: z.number().int().min(0).max(366), excludedCount: z.number().int().min(0).max(368), recordCount: z.number().int().min(0).max(366) });
export const cockpitMatrix = z.strictObject({ rows: z.array(cockpitRow).max(50), total: z.number().int().min(0).max(500), page: z.number().int().min(1).max(1000), summary: cockpitTotals, summaryComplete: z.boolean(), states: z.strictObject({ loss: z.number().int().min(0).max(500), break_even: z.number().int().min(0).max(500), recorded_complete: z.number().int().min(0).max(500), data_incomplete: z.number().int().min(0).max(500) }), capturedAt: stamp });
const cockpitRule = z.strictObject({ id: z.string().min(1).max(250), kind: z.enum(["period_loss", "data_missing", "goal_assessment"]), level: z.enum(["critical", "attention", "data"]), title: z.string().max(400), reason: z.string().max(1000), advice: z.string().max(1000), actionPath: z.string().max(256).refine(v => v === "/workbench/overview/goals" || /^\/workbench\/overview\/stores\?storeId=[0-9a-f-]{36}&startDate=\d{4}-\d{2}-\d{2}&endDate=\d{4}-\d{2}-\d{2}$/.test(v)), actionLabel: z.string().max(100), source: z.literal("manual"), capturedAt: stamp, storeId: cockpitID.optional(), period: cockpitPeriod, profit: safe.optional(), goalRevision: cockpitRevision.optional(), recordCount: z.number().int().min(0).max(366).optional(), gapCount: z.number().int().min(0).max(366).optional(), excludedCount: z.number().int().min(0).max(368).optional(), recordDigest: z.string().regex(/^[0-9a-f]{64}$/).optional(), goalBasis: z.strictObject({ target: money.positive(), normalBps: z.number().int().min(1).max(10000), attentionBps: z.number().int().min(1).max(10000), minimumMarginBps: z.number().int().min(0).max(10000).nullable(), evaluation: cockpitEvaluation }).optional() });
const observationEvidence=z.strictObject({storeId:cockpitID,syncId:cockpitID,status:z.string().max(32),observedAt:stamp.nullable(),start:stamp.nullable(),end:stamp.nullable(),errorCode:z.string().max(100)});
export const cockpitRules = z.strictObject({ rules: z.array(cockpitRule).max(50), total: z.number().int().min(0).max(501), page: z.number().int().min(1).max(1000), capturedAt: stamp, goalUnavailable: z.boolean(),observations:z.strictObject({state:z.enum(["unavailable","permission_required","no_active_store","available","incomplete","latest_incomplete"]),exceptional:z.number().int().min(0),complete:z.boolean(),sources:z.array(observationEvidence).max(500),latest:z.array(observationEvidence).max(500),actionPath:z.literal("/workbench/store-orders")}) });
export const cockpitReceipt = z.strictObject({ commandId: cockpitID, operation: z.enum(["fact_create", "fact_update", "goal_create", "goal_update", "goal_restore"]), id: cockpitID, revision: cockpitRevision, committedAt: stamp });
const cockpitFactInput = z.strictObject({ id: cockpitID, storeId: cockpitID, expectedRevision: expected, fact: z.strictObject({ period: cockpitPeriod, amounts: cockpitAmounts, note: z.string().max(4000) }) });
const cockpitGoalInput = z.strictObject({ id: cockpitID, expectedRevision: expected, goal: cockpitGoalConfig });
const cockpitRestoreInput = z.strictObject({ id: cockpitID, expectedRevision: cockpitRevision, sourceRevision: cockpitRevision });
export function cockpitEndpoint(path: string, method: string) {
 const pathname = path.split("?")[0];
 if (method === "POST") {
  if (pathname === "facts") return { input: cockpitFactInput, output: cockpitReceipt };
  if (pathname === "goals") return { input: cockpitGoalInput, output: cockpitReceipt };
  if (pathname === "goals/restore") return { input: cockpitRestoreInput, output: cockpitReceipt };
  return null;
 }
 if (method !== "GET") return null;
 let output: z.ZodType | undefined;
 if (pathname === "capabilities") output = cockpitCapabilities;
 if (pathname === "stores") output = cockpitMatrix;
 if (/^stores\/[0-9a-f-]{36}$/.test(pathname)) output = cockpitDetail;
 if (pathname === "goals") output = cockpitGoals;
 if (pathname === "goals/head") output = cockpitHead;
 if (pathname === "goals/history") output = z.array(cockpitGoal).max(20);
 if (pathname === "facts") output = z.array(cockpitRecord).max(50);
 if (/^facts\/[0-9a-f-]{36}$/.test(pathname)) output = cockpitRecord;
 if (/^facts\/[0-9a-f-]{36}\/history$/.test(pathname)) output = z.array(cockpitRecord).max(20);
 if (pathname === "alerts" || pathname === "advice") output = cockpitRules;
 return output ? { input: null, output } : null;
}
export type CockpitCapabilities = z.infer<typeof cockpitCapabilities>;
export type CockpitRecord = z.infer<typeof cockpitRecord>;
export type CockpitRule = z.infer<typeof cockpitRule>;
export type CockpitGoalConfig = z.infer<typeof cockpitGoalConfig>;

export function moneyCents(value: string): number | null {
 if (!/^(0|[1-9][0-9]{0,10})(\.[0-9]{1,2})?$/.test(value)) return null;
 const [whole, fraction = ""] = value.split("."), result = BigInt(whole) * BigInt("100") + BigInt(fraction.padEnd(2, "0"));
 return result <= BigInt("1000000000000") ? Number(result) : null;
}
export function centsInput(value: number): string { const n = BigInt(value); return `${n / BigInt("100")}.${String(n % BigInt("100")).padStart(2, "0")}`; }
export function moneyText(value: number): string { const n = BigInt(value), a = n < BigInt("0") ? -n : n; return `${n < BigInt("0") ? "-" : ""}¥${new Intl.NumberFormat("zh-CN").format(a / BigInt("100"))}.${String(a % BigInt("100")).padStart(2, "0")}`; }
export function ratioText(value: z.infer<typeof cockpitRational> | null | undefined): string { if (!value) return "—"; const n = BigInt(value.numerator) * BigInt("1000") / BigInt(value.denominator), a = n < BigInt("0") ? -n : n; return `${n < BigInt("0") ? "-" : ""}${a / BigInt("10")}.${a % BigInt("10")}%`; }
