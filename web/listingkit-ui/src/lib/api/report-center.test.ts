import { beforeEach, expect, it, vi } from "vitest";
import { reportResultSchema, reviewLocator, sourceRef } from "@/lib/contracts/report-center";
import { readReportIntent, reportRequest, writeReportIntent } from "./report-center";
const scope = { userId: "actor-a", organizationId: "org-a" };
const intent = { operation: "save" as const, key: "1510eced-9831-49de-a28c-098cb16deba1", source: { kind: "TITLE_REVIEW" as const, id: "1510eced-9831-49de-a28c-098cb16deba2", version: "1:pending" } };
beforeEach(() => { sessionStorage.clear(); vi.unstubAllGlobals(); });
it("preserves the source owner's canonical SHEIN digest", () => {
  const ref = { kind: "SHEIN_RECORD", id: intent.source.id, version: `sha256:${"a".repeat(64)}` };
  expect(sourceRef.safeParse(ref).success).toBe(true);
  expect(sourceRef.safeParse({ ...ref, version: "a".repeat(64) }).success).toBe(false);
});
it("uses only an exact current-app Review locator without accepting client content or version", () => {
  const origin = "http://localhost:3000", path = `/workbench/ai/tasks/pending/other?proposal_id=${intent.source.id}`;
  expect(reviewLocator(intent.source.id, origin)).toBe(intent.source.id);
  expect(reviewLocator(path, origin)).toBe(intent.source.id);
  expect(reviewLocator(origin + path, origin)).toBe(intent.source.id);
  for (const input of [`https://other.example${path}`, `${path}&version=2:applied`, `${path}&proposal_id=${intent.source.id}`, `${path}#body`, "/wrong?proposal_id=" + intent.source.id]) expect(reviewLocator(input, origin)).toBeNull();
});
it("retains the exact intent per actor and enterprise after a lost response", async () => {
  writeReportIntent(scope, intent); const fetch = vi.fn().mockRejectedValue(new Error("lost")); vi.stubGlobal("fetch", fetch);
  await expect(reportRequest(scope, "", reportResultSchema, undefined, intent)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" });
  expect(readReportIntent(scope)).toEqual(intent); expect(readReportIntent({ ...scope, organizationId: "other" })).toBeNull(); expect(readReportIntent({ ...scope, userId: "other" })).toBeNull();
  await expect(reportRequest(scope, "", reportResultSchema, undefined, readReportIntent(scope)!)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" });
  expect(new Headers(fetch.mock.calls[0][1].headers).get("Idempotency-Key")).toBe(intent.key); expect(fetch.mock.calls[0][1].body).toBe(fetch.mock.calls[1][1].body);
});
it("keeps colon-containing actor and enterprise scopes independent", () => {
  const first = { userId: "a:b", organizationId: "c" }, second = { userId: "a", organizationId: "b:c" };
  writeReportIntent(first, intent);
  expect(readReportIntent(second)).toBeNull();
  const other = { ...intent, key: "1510eced-9831-49de-a28c-098cb16deba3" };
  writeReportIntent(second, other);
  expect(readReportIntent(first)).toEqual(intent);
  expect(readReportIntent(second)).toEqual(other);
  writeReportIntent(second, null);
  expect(readReportIntent(first)).toEqual(intent);
});
it("refuses corrupt recovery data instead of inventing a new key", () => { writeReportIntent(scope, intent); sessionStorage.setItem(sessionStorage.key(0)!, "{}"); expect(() => readReportIntent(scope)).toThrow("INTENT_STORAGE_UNAVAILABLE"); });
