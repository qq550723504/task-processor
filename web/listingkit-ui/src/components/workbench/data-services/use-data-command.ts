"use client";
import { useEffect, useRef, useState } from "react";
import { DataAPIError, loadDataIntent, saveDataIntent, clearDataIntent, executeDataIntent, recoverDataIntent, validDataIntent, type DataScope, type DataIntent } from "@/lib/api/data-services";
export function useDataCommand(scope: DataScope, specialist = false) {
    const [pending, setPending] = useState<DataIntent | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState("");
    const active = useRef<AbortController | null>(null);
    useEffect(() => {
        let mounted = true;
        Promise.resolve().then(() => {
            if (mounted)
                setPending(loadDataIntent(scope, specialist));
        });
        return () => { mounted = false; active.current?.abort(); };
    }, [scope, specialist]);
    async function perform(intent: DataIntent, recover: boolean) {
        active.current?.abort();
        const controller = new AbortController();
        active.current = controller;
        setBusy(true);
        setError("");
        try {
            const value = await (recover ? recoverDataIntent(intent, controller.signal) : executeDataIntent(intent, controller.signal));
            if (controller.signal.aborted)
                return;
            clearDataIntent(scope, specialist);
            setPending(null);
            return value;
        }
        catch (e) {
            if (controller.signal.aborted)
                return;
            const code = e instanceof DataAPIError ? e.code : "DATA_UNKNOWN";
            setError(code);
            if (!recover && e instanceof DataAPIError && [400, 403, 404, 409].includes(e.status)) {
                clearDataIntent(scope, specialist);
                setPending(null);
            }
        }
        finally {
            if (!controller.signal.aborted)
                setBusy(false);
        }
    }
    async function run(path: string, body: unknown, binary?: string, headers?: Record<string, string>) {
        const original = loadDataIntent(scope, specialist);
        if (busy || pending || original) {
            if (original)
                setPending(original);
            setError("PENDING_COMMAND");
            return;
        }
        const intent: DataIntent = { ...scope, path, key: crypto.randomUUID(), specialist, ...(binary ? { binary, headers } : { body }) };
        if (!validDataIntent(intent)) {
            setError("INVALID_DATA_REQUEST");
            return;
        }
        if (!saveDataIntent(intent)) {
            setError("INTENT_STORAGE_UNAVAILABLE");
            return;
        }
        ;
        setPending(intent);
        return perform(intent, false);
    }
    return { pending, busy, error, run, recover: () => pending ? perform(pending, true) : Promise.resolve(undefined) };
}
