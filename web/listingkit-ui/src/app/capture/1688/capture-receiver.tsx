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

type ViewState = { entry: CaptureEntry | null; payload?: BrowserCapturePayload; started: boolean; busy: boolean; bound?: AcquisitionContext; result?: AcquisitionResult; message: string; refusal?: string };
const sameScope = (a: AcquisitionContext, b: AcquisitionContext) => a.userId === b.userId && a.organizationId === b.organizationId;
const unknownMessage = "Outcome is unknown. Check the original operation; do not submit a new capture.";
// A status the executor may record: `acquiring`/`prepared` are in-flight reads, not
// answers, and publishing one would let an unfinished operation be recorded as terminal.
const terminalStatuses = ["published", "failed", "outcome_unknown"];
// Machine-readable refusal codes for the batch executor (`data-batch-submit-refusal`).
// Design section 4 D1.2 guard 3: when this page reports that it dispatched nothing, the
// executor must never click again and must stop for a person. Each code names the state
// that produced it, so a stop is diagnosable without reading this page's prose.
const refusalContextUnavailable = "CONTEXT_UNAVAILABLE";
const refusalNotDispatched = "NOT_DISPATCHED";
// The two branches below are pre-attempt rather than click-time, but they are the same
// class: the page knows nothing was dispatched, so it must say so instead of letting the
// executor wait out its timeout and record a generic transfer failure.
const refusalHandoffUnreadable = "HANDOFF_UNREADABLE";
const refusalKeyUnretained = "OPERATION_KEY_UNRETAINED";

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
  // The executor reads the marked element's own text, never this value, so the marker is
  // bare — exactly as the fixture renders it. It is absent until the application has
  // verified a context, because an unverified account must not be readable as verified.
  const scopeMarker = ready ? "" : undefined;

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
        // "Nothing was submitted by this page" is definitive, not a guess: the frozen
        // payload never arrived, so no control was ever rendered to click.
        if (!controller.signal.aborted) setView((v) => ({ ...v, refusal: refusalHandoffUnreadable, message: "Browser handoff expired. Nothing was submitted by this page; request a new handoff in the extension." }));
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
      catch { busy.current = false; active.current = null; setView((v) => ({ ...v, refusal: refusalKeyUnretained, message: "Recovery key could not be retained. Nothing was submitted." })); return; }
      reserved.current = true;
    }
    setView((v) => ({ ...v, busy: true, started: true, bound: consent, result: undefined, refusal: undefined, message: "Confirming current access..." }));
    // `result` and `refusal` are cleared here, once per attempt. Each failure branch below
    // still states its own outcome rather than inheriting the previous attempt's.
    let dispatched = false;
    const stillCurrent = () => {
      const c = current.current;
      return !controller.signal.aborted && !c.isLoading && !c.isSwitching && !c.error && !c.blockingError && c.user?.id === consent.userId && c.effectiveOrganization?.id === consent.organizationId;
    };
    try {
      const refreshed = await context.retry();
      if (!refreshed || refreshed.selectionRequired || refreshed.user.id !== consent.userId || refreshed.effectiveOrganizationId !== consent.organizationId || !stillCurrent()) {
        // No capture request was dispatched. Keep the in-memory payload
        // submittable after the user restores the original context.
        reserved.current = false;
        if (mounted.current) setView((v) => ({ ...v, started: false, bound: undefined, result: undefined, refusal: refusalContextUnavailable, message: "Context changed or access is unavailable. No new request was dispatched; return to the original account and enterprise to try again." }));
        return;
      }
      if (entry.kind === "handoff" && submit) void notifyCaptureStatus(entry, "processing");
      setView((v) => ({ ...v, message: submit ? "Submitting the frozen capture..." : "Checking the original operation..." }));
      let result: AcquisitionResult;
      if (submit && body) {
        dispatched = true;
        result = await capture1688(Object.freeze({ ...consent, key: entry.key, body }), controller.signal);
      } else if (operationId) {
        result = await readBrowserCapture(consent, operationId, controller.signal);
      } else {
        result = await readBrowserCaptureByKey(consent, entry.key, controller.signal);
      }
      if (!mounted.current || !stillCurrent()) return;
      setView((v) => ({ ...v, refusal: undefined, result, message: result.outcome === "published" ? `Published version ${result.catalogVersion}` : result.outcome === "failed" ? "The backend recorded this operation as failed." : unknownMessage }));
      if (entry.kind === "handoff") void notifyCaptureStatus(entry, result.outcome === "published" ? "published" : result.outcome === "failed" ? "failed" : "outcome_unknown", result.operationId);
    } catch (failure) {
      const code = failure instanceof WorkbenchContextError ? failure.code : "OUTCOME_UNKNOWN";
      if (submit && code === "ACQUISITION_CAPACITY") {
        reserved.current = false;
        if (mounted.current) setView((v) => ({ ...v, started: false, bound: undefined, result: undefined, refusal: code, message: "Capture was not admitted because active capacity is full. No operation was created; try again when capacity is available." }));
        if (entry.kind === "handoff") void notifyCaptureStatus(entry, "failed");
        return;
      }
      if (submit && code === "DEADLINE_EXCEEDED") {
        reserved.current = false;
        if (mounted.current) setView((v) => ({ ...v, started: false, bound: undefined, result: undefined, refusal: code, message: "Capture was not submitted before the deadline. No operation was created; try again with the same handoff." }));
        if (entry.kind === "handoff") void notifyCaptureStatus(entry, "failed");
        return;
      }
      if (submit && ["INVALID_ACQUISITION", "SOURCE_TOO_LARGE"].includes(code)) {
        reserved.current = false;
        if (mounted.current) setView((v) => ({ ...v, payload: undefined, started: false, bound: undefined, result: undefined, refusal: code, message: "Capture was rejected before admission. No operation was created; start a new handoff from the extension." }));
        if (entry.kind === "handoff") void notifyCaptureStatus(entry, "failed");
        return;
      }
      if (!dispatched && submit) {
        reserved.current = false;
        if (mounted.current) setView((v) => ({ ...v, started: false, bound: undefined, result: undefined, refusal: refusalNotDispatched, message: "Capture was not submitted. Restore the original account and enterprise, then try again." }));
        return;
      }
      if (!mounted.current || !stillCurrent()) return;
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
  const terminal = receipt && terminalStatuses.includes(receipt.outcome) ? receipt : undefined;
  return <section className="mx-auto w-full max-w-3xl space-y-6 py-6">
    <header><p className="text-sm text-muted-foreground">1688 / Browser capture</p><h1 className="mt-2 text-2xl font-semibold">Confirm a browser capture</h1><p className="mt-2 text-sm text-muted-foreground">Observed page facts only. Missing facts remain unknown; the backend controls publication.</p></header>
    <div className="grid gap-4 rounded-xl border border-border bg-card p-5 sm:grid-cols-2"><div><p className="text-xs text-muted-foreground">Current verified account</p><p className="break-all" data-batch-scope-actor={scopeMarker}>{ready ? context.user?.id : "Unavailable"}</p></div><div><p className="text-xs text-muted-foreground">Effective enterprise</p><p>{ready ? context.effectiveOrganization?.name : "Unavailable"}</p><p className="break-all text-xs text-muted-foreground" data-batch-scope-organization={scopeMarker}>{ready ? context.effectiveOrganization?.id : null}</p></div></div>
    {view.payload && !view.started ? <section className="space-y-3 rounded-xl border border-border bg-card p-5"><h2 className="break-words text-lg font-medium">{view.payload.evidence.title ?? "Title not observed"}</h2><p className="break-all text-sm text-muted-foreground">{view.payload.evidence.sourceURL}</p><p className="text-sm">{view.payload.evidence.warnings.length} warnings; {view.payload.evidence.missingFacts.length} missing facts</p><ul className="space-y-1 text-sm text-muted-foreground">{view.payload.evidence.missingFacts.map((fact, index) => <li key={index}>{fact.field}: {fact.reason}</li>)}</ul></section> : null}
    <p role="status" className="break-words rounded-xl border border-border bg-muted/40 p-4 text-sm">{scoped ? view.message : "Context changed or access is unavailable. Return to the original account and enterprise; old receipts are hidden."}</p>
    {/* The batch executor reads this page without reading its prose: the verified scope,
        the real control, this page's own refusal, and a terminal result it may record.
        Machine-readable only — hidden text adds nothing a person reads. */}
    <div hidden data-batch-submit-result={terminal ? terminal.outcome : ""} data-batch-operation-id={terminal?.operationId}></div>
    <div hidden data-batch-submit-refusal>{view.refusal ?? ""}</div>
    {receipt?.outcome === "published" ? <dl className="space-y-2 rounded-xl border border-border bg-card p-5 text-sm"><dt>Product</dt><dd className="break-all">{receipt.productKey}</dd><dt>Original publication</dt><dd className="break-all">{receipt.publicationId}</dd><dt>Warnings / missing facts</dt><dd>{receipt.warnings.length} / {receipt.missingFacts.length}</dd></dl> : null}
    <div className="flex flex-wrap gap-3">
      {!view.started && view.payload ? <Button data-batch-confirm-submit="" disabled={!scoped || view.busy} onClick={() => void run(true)}>Confirm and submit</Button> : null}
      {view.started && view.entry ? <Button disabled={!scoped || view.busy} variant="outline" onClick={() => void run(false)}>Check original operation</Button> : null}
      {view.busy ? <Button variant="outline" onClick={cancel}>Stop waiting</Button> : null}
    </div>
    {view.started ? <p className="text-xs text-muted-foreground">Keep this page to recover the original operation. Reloading never submits again. Do not share its private recovery link.</p> : null}
  </section>;
}
