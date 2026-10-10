import { beforeEach, expect, it, vi } from "vitest";
import { reportResultSchema } from "@/lib/contracts/report-center";
import { readReportIntent, reportRequest, writeReportIntent } from "./report-center";
const scope = { userId: "actor-a", organizationId: "org-a" };
const intent = { operation: "save" as const, key: "1510eced-9831-49de-a28c-098cb16deba1", source: { kind: "TITLE_REVIEW" as const, id: "1510eced-9831-49de-a28c-098cb16deba2", version: "1:pending" } };
beforeEach(() => { sessionStorage.clear(); vi.unstubAllGlobals(); });
it("retains the exact intent per actor and enterprise after a lost response", async () => {
  writeReportIntent(scope, intent); const fetch = vi.fn().mockRejectedValue(new Error("lost")); vi.stubGlobal("fetch", fetch);
  await expect(reportRequest(scope, "", reportResultSchema, undefined, intent)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" });
  expect(readReportIntent(scope)).toEqual(intent); expect(readReportIntent({ ...scope, organizationId: "other" })).toBeNull(); expect(readReportIntent({ ...scope, userId: "other" })).toBeNull();
  await expect(reportRequest(scope, "", reportResultSchema, undefined, readReportIntent(scope)!)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" });
  expect(new Headers(fetch.mock.calls[0][1].headers).get("Idempotency-Key")).toBe(intent.key); expect(fetch.mock.calls[0][1].body).toBe(fetch.mock.calls[1][1].body);
});
it("refuses corrupt recovery data instead of inventing a new key", () => { sessionStorage.setItem(`personal-report-intent:${scope.userId}:${scope.organizationId}`, "{}"); expect(() => readReportIntent(scope)).toThrow("INTENT_STORAGE_UNAVAILABLE"); });
