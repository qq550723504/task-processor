"use client";

import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { capture1688, readBrowserCapture, readBrowserCaptureByKey } from "@/lib/api/browser-capture";
import type { AcquisitionContext } from "@/lib/api/product-acquisition";
import { WorkbenchContextError } from "@/lib/api/workbench-context";
import type { BrowserCapturePayload } from "@/lib/contracts/browser-capture";
import type { AcquisitionResult } from "@/lib/contracts/product-acquisition";
import { parseCaptureEntry, readCaptureHandoff, notifyCaptureStatus, type CaptureEntry } from "./capture-handoff";

type ViewState = { entry: CaptureEntry | null; payload?: BrowserCapturePayload; started: boolean; busy: boolean; bound?: AcquisitionContext; result?: AcquisitionResult; message: string };
const sameScope = (a: AcquisitionContext, b: AcquisitionContext) => a.userId === b.userId && a.organizationId === b.organizationId;
const unknownMessage = "Outcome is unknown. Check the original operation; do not submit a new capture.";

export function CaptureReceiver() {
  const context = useWorkbenchContext();
  const [view, setView] = useState<ViewState>({ entry: null, started: false, busy: false, message: "Reading the browser handoff..." });
  const current = useRef(context);
  const active = useRef<AbortController | null>(null);
  const mounted = useRef(false);
  const reserved = useRef(false);
  const busy = useRef(false);
  const ready = !context.isLoading && !context.isSwitching && !context.selectionRequired && !context.error && !context.blockingError && !!context.user && !!context.effectiveOrganization;
  const scope = { userId: context.user?.id ?? "", organizationId: context.effectiveOrganization?.id ?? "" };
  const scoped = ready && (!view.bound || sameScope(view.bound, scope));

  useEffect(() => {
    current.current = context;
    if (!scoped) active.current?.abort();
  }, [context, scoped]);
  useEffect(() => {
    mounted.current = true;
    const controller = new AbortController();
    void (async () => {
      const entry = parseCaptureEntry(window.location.href);
      // Yield before state changes and discard a StrictMode/unmounted read.
      await Promise.resolve();
      if (controller.signal.aborted) return;
      if (!entry) { setView((v) => ({ ...v, message: "This handoff is invalid or expired. Start a new handoff from the extension only if no request was submitted." })); return; }
      if (entry.kind === "recovery") { reserved.current = true; setView((v) => ({ ...v, entry, started: true, message: unknownMessage })); return; }
      try {
        const payload = await readCaptureHandoff(entry, controller.signal);
        if (!controller.signal.aborted) setView((v) => ({ ...v, entry, payload, message: "Confirm the current account and enterprise before submitting." }));
      } catch {
        if (!controller.signal.aborted) setView((v) => ({ ...v, message: "Browser handoff expired. Nothing was submitted by this page; request a new handoff in the extension." }));
      }
    })();
    return () => { mounted.current = false; controller.abort(); active.current?.abort(); };
  }, []);

  async function run(submit: boolean) {
    const entry = view.entry;
    if (!entry || !scoped || busy.current || (submit && (reserved.current || !view.payload))) return;
    const consent = Object.freeze({ ...scope });
    const operationId = view.result?.operationId;
    const body = view.payload ? JSON.stringify(view.payload) : undefined;
    const controller = new AbortController(); active.current = controller; busy.current = true;
    if (submit) {
      // The fragment stores only the original key. No payload/identity/storage/logging.
      try { window.history.replaceState(null, "", `/capture/1688#operationKey=${entry.key}`); }
      catch { busy.current = false; active.current = null; setView((v) => ({ ...v, message: "Recovery key could not be retained. Nothing was submitted." })); return; }
      reserved.current = true;
    }
    setView((v) => ({ ...v, busy: true, started: true, bound: consent, result: undefined, message: "Confirming current access..." }));
    const stillCurrent = () => {
      const c = current.current;
      return !controller.signal.aborted && !c.isLoading && !c.isSwitching && !c.error && !c.blockingError && c.user?.id === consent.userId && c.effectiveOrganization?.id === consent.organizationId;
    };
    try {
      const refreshed = await context.retry();
      if (!refreshed || refreshed.selectionRequired || refreshed.user.id !== consent.userId || refreshed.effectiveOrganizationId !== consent.organizationId || !stillCurrent()) {
        if (mounted.current) setView((v) => ({ ...v, message: "Context changed or access is unavailable. No new request was dispatched; return to the original account and enterprise to check its operation." }));
        return;
      }
      if (entry.kind === "handoff" && submit) void notifyCaptureStatus(entry, "processing");
      setView((v) => ({ ...v, message: submit ? "Submitting the frozen capture..." : "Checking the original operation..." }));
      const result = submit && body
        ? await capture1688(Object.freeze({ ...consent, key: entry.key, body }), controller.signal)
        : operationId
          ? await readBrowserCapture(consent, operationId, controller.signal)
          : await readBrowserCaptureByKey(consent, entry.key, controller.signal);
      if (!mounted.current || !stillCurrent()) return;
      setView((v) => ({ ...v, result, message: result.outcome === "published" ? `Published version ${result.catalogVersion}` : result.outcome === "failed" ? "The backend recorded this operation as failed." : unknownMessage }));
      if (entry.kind === "handoff") void notifyCaptureStatus(entry, result.outcome === "published" ? "published" : result.outcome === "failed" ? "failed" : "outcome_unknown", result.operationId);
    } catch (failure) {
      if (!mounted.current || !stillCurrent()) return;
      const code = failure instanceof WorkbenchContextError ? failure.code : "OUTCOME_UNKNOWN";
      const message = code === "ACQUISITION_NOT_FOUND" ? "No operation is visible for this key in the current account and enterprise. This does not prove a prior request failed. No new submission will be made."
        : ["AUTHENTICATION_REQUIRED", "FORBIDDEN", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED", "ORGANIZATION_CONTEXT_CHANGED", "IDENTITY_CONTEXT_CHANGED"].includes(code) ? "Access is unavailable or context changed. Keep this recovery page and return with the original account and enterprise."
        : code === "IDEMPOTENCY_CONFLICT" ? "This key belongs to a different intent. Do not replace it or retry with a new key." : unknownMessage;
      setView((v) => ({ ...v, message }));
      if (entry.kind === "handoff") void notifyCaptureStatus(entry, "outcome_unknown");
    } finally {
      busy.current = false;
      if (active.current === controller) active.current = null;
      if (mounted.current) setView((v) => ({ ...v, busy: false }));
    }
  }
  function cancel() {
    active.current?.abort();
    setView((v) => ({ ...v, result: undefined, message: unknownMessage }));
  }

  const receipt = scoped ? view.result : undefined;
  return <section className="mx-auto w-full max-w-3xl space-y-6 py-6">
    <header><p className="text-sm text-muted-foreground">1688 / Browser capture</p><h1 className="mt-2 text-2xl font-semibold">Confirm a browser capture</h1><p className="mt-2 text-sm text-muted-foreground">Observed page facts only. Missing facts remain unknown; the backend controls publication.</p></header>
    <div className="grid gap-4 rounded-xl border border-border bg-card p-5 sm:grid-cols-2"><div><p className="text-xs text-muted-foreground">Current verified account</p><p className="break-all">{ready ? context.user?.id : "Unavailable"}</p></div><div><p className="text-xs text-muted-foreground">Effective enterprise</p><p>{ready ? context.effectiveOrganization?.name : "Unavailable"}</p><p className="break-all text-xs text-muted-foreground">{ready ? context.effectiveOrganization?.id : null}</p></div></div>
    {view.payload && !view.started ? <section className="space-y-3 rounded-xl border border-border bg-card p-5"><h2 className="break-words text-lg font-medium">{view.payload.evidence.title ?? "Title not observed"}</h2><p className="break-all text-sm text-muted-foreground">{view.payload.evidence.sourceURL}</p><p className="text-sm">{view.payload.evidence.warnings.length} warnings; {view.payload.evidence.missingFacts.length} missing facts</p><ul className="space-y-1 text-sm text-muted-foreground">{view.payload.evidence.missingFacts.map((fact, index) => <li key={index}>{fact.field}: {fact.reason}</li>)}</ul></section> : null}
    <p role="status" className="break-words rounded-xl border border-border bg-muted/40 p-4 text-sm">{scoped ? view.message : "Context changed or access is unavailable. Return to the original account and enterprise; old receipts are hidden."}</p>
    {receipt?.outcome === "published" ? <dl className="space-y-2 rounded-xl border border-border bg-card p-5 text-sm"><dt>Product</dt><dd className="break-all">{receipt.productKey}</dd><dt>Original publication</dt><dd className="break-all">{receipt.publicationId}</dd><dt>Warnings / missing facts</dt><dd>{receipt.warnings.length} / {receipt.missingFacts.length}</dd></dl> : null}
    <div className="flex flex-wrap gap-3">
      {!view.started && view.payload ? <Button disabled={!scoped || view.busy} onClick={() => void run(true)}>Confirm and submit</Button> : null}
      {view.started && view.entry ? <Button disabled={!scoped || view.busy} variant="outline" onClick={() => void run(false)}>Check original operation</Button> : null}
      {view.busy ? <Button variant="outline" onClick={cancel}>Stop waiting</Button> : null}
    </div>
    {view.started ? <p className="text-xs text-muted-foreground">Keep this page to recover the original operation. Reloading never submits again. Do not share its private recovery link.</p> : null}
  </section>;
}
