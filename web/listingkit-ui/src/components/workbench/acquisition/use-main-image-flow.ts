"use client";

import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { AcquisitionAPIError, type AcquisitionContext } from "@/lib/api/product-acquisition";
import { approveMainImage, readMainImage, readMainImageCandidates, startMainImage } from "@/lib/api/acquisition-main-image";
import { mainImageApprovalRequestSchema, mainImageStartRequestSchema, type MainImageCandidate, type MainImageResult } from "@/lib/contracts/acquisition-main-image";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";

const intentSchema = mainImageStartRequestSchema.extend({
  key: z.string().refine(isAcquisitionUUID),
  runId: z.string().refine(isAcquisitionUUID).optional(),
  approval: mainImageApprovalRequestSchema.optional(),
}).strict().refine((value) => !value.approval || !!value.runId);
type Intent = z.infer<typeof intentSchema>;
type View = {
  loaded: boolean; candidates: MainImageCandidate[] | null; selected: string;
  intent: Intent | null; result: MainImageResult | null; busy: boolean; error: string;
  approvalAcknowledged: boolean;
};
const emptyView = (): View => ({ loaded: false, candidates: null, selected: "", intent: null, result: null, busy: false, error: "", approvalAcknowledged: false });
type Session = { key: string; alive: boolean; storageReady: boolean; inFlight: boolean; view: View; requests: Set<AbortController> };

// Browser state holds only the original command identities. It is not evidence
// of generation, approval or permission; every read/replay uses the typed API.
// Layout remains separate while the exact Figma result view is being confirmed.
export function useMainImageFlow({ operationId, scope }: { operationId: string; scope: AcquisitionContext }) {
  const { userId, organizationId } = scope;
  const storageKey = `main-image:${JSON.stringify([userId, organizationId, operationId])}`;
  const current = useRef<Session | null>(null);
  const [snapshot, setSnapshot] = useState<{ key: string; view: View }>({ key: storageKey, view: emptyView() });
  const view = snapshot.key === storageKey ? snapshot.view : emptyView();

  function update(session: Session, next: Partial<View>) {
    if (!session.alive || current.current !== session) return;
    session.view = { ...session.view, ...next };
    setSnapshot({ key: session.key, view: session.view });
  }
  function failure(session: Session, error: unknown) {
    const code = error instanceof AcquisitionAPIError ? error.code : "IMAGE_UNAVAILABLE";
    const denied = error instanceof AcquisitionAPIError && (error.status === 401 || error.status === 403 || code === "IDENTITY_CONTEXT_CHANGED" || code === "ORGANIZATION_CONTEXT_CHANGED");
    update(session, { error: code, ...(denied ? { result: null, candidates: null, approvalAcknowledged: false, busy: false } : {}) });
    if (denied) {
      // A concurrent read may ignore abort or already be resolved. Retire this
      // local session as well; only a fresh verified context may read again.
      session.alive = false;
      session.requests.forEach((controller) => controller.abort());
    }
  }
  function persist(session: Session, intent: Intent): boolean {
    try {
      sessionStorage.setItem(session.key, JSON.stringify(intent));
      if (sessionStorage.getItem(session.key) !== JSON.stringify(intent)) throw new Error("storage unavailable");
      update(session, { intent });
      return true;
    } catch {
      update(session, { error: "LOCAL_STORAGE_UNAVAILABLE" });
      return false;
    }
  }

  useEffect(() => {
    const session: Session = { key: storageKey, alive: true, storageReady: false, inFlight: false, view: emptyView(), requests: new Set() };
    current.current = session;
    let restored: Intent | null = null;
    try {
      const raw = sessionStorage.getItem(storageKey);
      if (raw !== null) {
        // Never erase an unreadable original command and silently mint a new ID.
        const parsed = intentSchema.safeParse(JSON.parse(raw));
        if (!parsed.success) throw new Error("invalid local intent");
        restored = parsed.data;
      }
      session.storageReady = true;
      update(session, { loaded: true, intent: restored, selected: restored?.sourceImageId ?? "" });
    } catch { update(session, { loaded: true, error: "LOCAL_STORAGE_UNAVAILABLE" }); }
    const request = new AbortController(); session.requests.add(request);
    const context = { userId, organizationId };
    void readMainImageCandidates(operationId, context, request.signal)
      .then((value) => update(session, { candidates: value.candidates }))
      .catch((error: unknown) => failure(session, error));
    if (restored?.runId) {
      void readMainImage(operationId, restored.runId, context, request.signal)
        .then((result) => update(session, { result }))
        .catch((error: unknown) => failure(session, error));
    }
    return () => { session.alive = false; session.requests.forEach((controller) => controller.abort()); };
    // Only primitive identity changes create a session, not parent rerenders.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [operationId, userId, organizationId, storageKey]);

  function sessionForAction() {
    const session = current.current;
    return session?.alive && session.key === storageKey && !session.inFlight ? session : null;
  }
  async function execute(session: Session, action: (signal: AbortSignal) => Promise<void>) {
    session.inFlight = true; update(session, { busy: true, error: "" });
    const request = new AbortController(); session.requests.add(request);
    try { await action(request.signal); }
    catch (error) { failure(session, error); }
    finally { session.requests.delete(request); session.inFlight = false; update(session, { busy: false }); }
  }
  function live(session: Session) { return session.alive && current.current === session; }
  function select(id: string) {
    const session = sessionForAction();
    if (session && !session.view.intent && session.view.candidates?.some((candidate) => candidate.id === id)) update(session, { selected: id });
  }
  async function start() {
    const session = sessionForAction();
    if (!session || !session.storageReady) return;
    const { intent, selected, candidates } = session.view;
    if (intent?.runId || (!intent && !candidates?.some((candidate) => candidate.id === selected))) return;
    const next = intent ?? { sourceImageId: selected, key: crypto.randomUUID() };
    if (!persist(session, next)) return;
    await execute(session, async (signal) => {
      const accepted = await startMainImage(operationId, next.sourceImageId, next.key, { userId, organizationId }, signal);
      if (!live(session) || !persist(session, { ...next, runId: accepted.runId })) return;
      update(session, { result: await readMainImage(operationId, accepted.runId, { userId, organizationId }, signal) });
    });
  }
  async function refresh() {
    const session = sessionForAction(), runId = session?.view.intent?.runId;
    if (!session || !runId) return;
    await execute(session, async (signal) => update(session, { result: await readMainImage(operationId, runId, { userId, organizationId }, signal) }));
  }
  async function approve() {
    const session = sessionForAction();
    if (!session || !session.storageReady) return;
    const { intent, result } = session.view;
    if (!intent?.runId || !result) return;
    const existing = intent.approval;
    if (!existing && (!result.approvalAvailable || !result.imageUrl)) return;
    const approval = existing ?? { planRevision: result.planRevision, resultDigest: result.resultDigest, actionId: crypto.randomUUID() };
    if (approval.planRevision !== result.planRevision || approval.resultDigest !== result.resultDigest) { update(session, { error: "IMAGE_CONFLICT" }); return; }
    if (!persist(session, { ...intent, approval })) return;
    await execute(session, async (signal) => {
      // Completed alone is not proof for this action. The existing API verifies
      // its exact immutable receipt when replaying this same payload.
      await approveMainImage(operationId, intent.runId!, approval, { userId, organizationId }, signal);
      if (!live(session)) return;
      update(session, { approvalAcknowledged: true });
      update(session, { result: await readMainImage(operationId, intent.runId!, { userId, organizationId }, signal) });
    });
  }
  return { ...view, select, start, refresh, approve };
}
