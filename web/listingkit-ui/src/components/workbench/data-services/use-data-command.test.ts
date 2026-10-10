import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { loadDataIntent, saveDataIntent } from "@/lib/api/data-services";
import { useDataCommand } from "./use-data-command";

afterEach(() => { cleanup(); sessionStorage.clear(); vi.unstubAllGlobals(); });

describe("original Amazon creation recovery without Collection read permission", () => {
    it.each([false, true])("replays the original write and clears pending only after success, restored=%s", async restored => {
        const scope = { userId: "writer", organizationId: "original-org" };
        const body = { query: { site: "us", mode: "asin", asins: ["B000123456"], limit: 1 }, maximumRows: 1, maximumCostFen: 5 };
        const stored = { ...scope, key: "a233d58b-1fd3-40d7-a983-d35bbec45313", path: "amazon/jobs", specialist: false, body };
        if (restored) expect(saveDataIntent(stored)).toBe(true);
        const writes: { body: string; key: string | null; user: string | null; org: string | null }[] = [];
        const reads: string[] = [];
        const failures = restored ? [503, 403] : [503, 503, 403];
        vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
            if (init?.method !== "POST") {
                reads.push(url);
                return Response.json({ error: { code: "FORBIDDEN" } }, { status: 403 });
            }
            expect(url).toBe("/api/workbench/data-services/amazon/jobs");
            const headers = new Headers(init.headers);
            writes.push({ body: String(init.body), key: headers.get("Idempotency-Key"), user: headers.get("X-Expected-User-ID"), org: headers.get("X-Expected-Organization-ID") });
            const failure = failures.shift();
            if (failure) return Response.json({ error: { code: failure === 503 ? "DATA_UNKNOWN" : "FORBIDDEN" } }, { status: failure });
            return Response.json({ id: stored.key, commandKey: headers.get("Idempotency-Key"), query: JSON.parse(String(init.body)).query, state: "ADMITTED", discovered: false, canceled: false, saved: 0, failed: 0, pending: 0, confirmedFen: 0, pendingFen: 0, createdAt: "2026-10-10T00:00:00Z", deadline: "2026-10-10T00:30:00Z" }, { status: 202 });
        }));
        const { result } = renderHook(() => useDataCommand(scope));
        await act(async () => { await Promise.resolve(); });
        if (!restored) await act(async () => { await result.current.run("amazon/jobs", body); });
        await waitFor(() => expect(result.current.pending).not.toBeNull());
        const original = result.current.pending!;
        expect(loadDataIntent(scope, false)).toEqual(original);

        for (const code of ["DATA_UNKNOWN", "FORBIDDEN"]) {
            await act(async () => { await result.current.recover(); });
            expect(result.current.error).toBe(code);
            expect(result.current.pending).toEqual(original);
            expect(loadDataIntent(scope, false)).toEqual(original);
        }
        await act(async () => { await result.current.recover(); });
        expect(result.current.error).toBe("");
        expect(result.current.pending).toBeNull();
        expect(loadDataIntent(scope, false)).toBeNull();
        expect(reads).toEqual([]);
        expect(writes).toHaveLength(restored ? 3 : 4);
        for (const write of writes) expect(write).toEqual({ body: JSON.stringify(body), key: original.key, user: scope.userId, org: scope.organizationId });

        await act(async () => { await result.current.run("amazon/jobs", { ...body, query: { ...body.query, asins: ["B000654321"] } }); });
        expect(writes.at(-1)?.key).not.toBe(original.key);
        expect(JSON.parse(writes.at(-1)!.body).query.asins).toEqual(["B000654321"]);
        expect(result.current.pending).toBeNull();
    });
});
