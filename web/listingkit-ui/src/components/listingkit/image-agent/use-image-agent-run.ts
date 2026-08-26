"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  approveImageAgentResults,
  cancelImageAgentRun,
  getImageAgentRun,
  imageAgentEventsUrl,
  replaceImageAgentPlan,
  resumeImageAgentCommand,
  retryImageAgentSlot,
} from "@/lib/api/image-agent";
import { ApiError } from "@/lib/api/api-error";
import type { ImageAgentPlan, ImageAgentProjection, ImageAgentProjectionEvent, ImageAgentSlot } from "@/lib/types/image-agent";

const baseReconnectDelayMs = 500;
const maxReconnectDelayMs = 30_000;
type PlanDraft = Pick<ImageAgentPlan, "source_asset_ids" | "style_reference_ids" | "slots">;

export function useImageAgentRun({ runId, initialRun }: { runId: string; initialRun?: ImageAgentProjection }) {
  const validInitial = initialRun?.run.id === runId ? initialRun : undefined;
  const [projection, setProjection] = useState<ImageAgentProjection | undefined>(validInitial);
  const [isLoading, setIsLoading] = useState(!validInitial);
  const [error, setError] = useState<string>();
  const [pendingAction, setPendingAction] = useState<string>();
  const projectionRef = useRef(validInitial);
  const cursorRef = useRef(validInitial?.last_event_id ?? 0);
  const versionRef = useRef(validInitial?.projection_version ?? 0);
  const mountedRef = useRef(true);
  const requestSequenceRef = useRef(0);
  const committedRequestRef = useRef(0);

  const commitSnapshot = useCallback((next: ImageAgentProjection, requestSequence = 0) => {
    if (!mountedRef.current || next.run.id !== runId) return;
    if (requestSequence > 0 && requestSequence < committedRequestRef.current) return;
    committedRequestRef.current = Math.max(committedRequestRef.current, requestSequence);
    projectionRef.current = next;
    cursorRef.current = next.last_event_id;
    versionRef.current = next.projection_version;
    setProjection(next);
    setIsLoading(false);
  }, [runId]);

  const refreshSnapshot = useCallback(async () => {
    const requestSequence = ++requestSequenceRef.current;
    const next = await getImageAgentRun(runId);
    commitSnapshot(next, requestSequence);
    return next;
  }, [commitSnapshot, runId]);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  useEffect(() => {
    if (initialRun?.run.id === runId) commitSnapshot(initialRun);
  }, [commitSnapshot, initialRun, runId]);

  useEffect(() => {
    let disposed = false;
    let source: EventSource | undefined;
    let listener: EventListener | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let recovering = false;
    let failureCount = 0;

    const closeSource = () => {
      if (!source) return;
      if (listener) source.removeEventListener("projection", listener);
      source.close();
      source = undefined;
      listener = undefined;
    };
    const clearTimer = () => {
      if (timer) clearTimeout(timer);
      timer = undefined;
    };
    const connect = () => {
      if (disposed || source) return;
      source = new EventSource(imageAgentEventsUrl(runId));
      listener = (raw: Event) => {
        const event = raw as MessageEvent<string>;
        const cursor = Number.parseInt(event.lastEventId, 10);
        const envelope = parseProjectionEvent(event.data);
        if (!envelope || !Number.isSafeInteger(cursor) || cursor <= 0) return;
        if (cursor <= cursorRef.current || envelope.projection_version <= versionRef.current) return;
        void recover();
      };
      source.addEventListener("projection", listener);
      source.onerror = () => { void recover(); };
    };
    const scheduleRetry = () => {
      if (disposed || timer) return;
      const exponential = Math.min(maxReconnectDelayMs, baseReconnectDelayMs * 2 ** failureCount);
      const jittered = Math.min(maxReconnectDelayMs, Math.round(exponential * (0.5 + Math.random())));
      failureCount += 1;
      timer = setTimeout(() => {
        timer = undefined;
        void recover();
      }, jittered);
    };
    async function recover() {
      if (disposed || recovering) return;
      recovering = true;
      closeSource();
      clearTimer();
      try {
        await refreshSnapshot();
        failureCount = 0;
        if (!disposed) {
          setError(undefined);
          connect();
        }
      } catch (cause) {
        if (!disposed) {
          setIsLoading(false);
          if (isAuthenticationError(cause)) {
            setError("图片 Agent 会话需要重新认证，请登录后重试");
          } else {
            setError(errorMessage(cause));
            scheduleRetry();
          }
        }
      } finally {
        recovering = false;
      }
    }

    if (projectionRef.current?.run.id === runId) connect();
    else void recover();
    return () => {
      disposed = true;
      closeSource();
      clearTimer();
    };
  }, [refreshSnapshot, runId]);

  const executeCommand = useCallback(async (key: string, execute: (actionID: string) => Promise<void>) => {
    const actionID = newActionID();
    setPendingAction(key);
    setError(undefined);
    try {
      await execute(actionID);
      await refreshSnapshot();
    } catch (cause) {
      if (mountedRef.current) setError(errorMessage(cause));
      try {
        await refreshSnapshot();
      } catch (refreshCause) {
        if (mountedRef.current) setError(`${errorMessage(cause)}；状态刷新失败：${errorMessage(refreshCause)}`);
      }
    } finally {
      if (mountedRef.current) setPendingAction(undefined);
    }
  }, [refreshSnapshot]);

  const retrySlot = useCallback(async (slotID: string) => {
    const current = projectionRef.current;
    if (!current) return;
    await executeCommand(`retry:${slotID}`, (actionID) => retryImageAgentSlot(runId, slotID, current.plan.revision, actionID));
  }, [executeCommand, runId]);

  const approveResults = useCallback(async () => {
    const current = projectionRef.current;
    if (!current?.result_digest) return;
    await executeCommand("approve", (actionID) => approveImageAgentResults(runId, current.plan.revision, current.result_digest!, actionID));
  }, [executeCommand, runId]);

  const cancel = useCallback(async () => {
    const current = projectionRef.current;
    if (!current) return;
    await executeCommand("cancel", (actionID) => cancelImageAgentRun(runId, current.plan.revision, actionID));
  }, [executeCommand, runId]);

  const replacePlan = useCallback(async (draft: PlanDraft) => {
    const current = projectionRef.current;
    if (!current) return;
    const expectedRevision = current.plan.revision;
    await executeCommand("replace-plan", (actionID) => {
      const plan: ImageAgentPlan = {
        ...current.plan,
        revision: expectedRevision + 1,
        parent_revision: expectedRevision,
        idempotency_key: `image-agent-plan-${actionID}`,
        source_asset_ids: [...draft.source_asset_ids],
        style_reference_ids: draft.style_reference_ids ? [...draft.style_reference_ids] : undefined,
        slots: draft.slots.map(cloneSlot),
      };
      return replaceImageAgentPlan(runId, expectedRevision, plan, actionID);
    });
  }, [executeCommand, runId]);

  const resumePending = useCallback(async () => {
    const receipt = projectionRef.current?.pending_command;
    if (!receipt) return;
    setPendingAction(`resume:${receipt.action_id}`);
    setError(undefined);
    try {
      await resumeImageAgentCommand(runId, receipt.action_id);
      await refreshSnapshot();
    } catch (cause) {
      if (mountedRef.current) setError(errorMessage(cause));
    } finally {
      if (mountedRef.current) setPendingAction(undefined);
    }
  }, [refreshSnapshot, runId]);

  return { projection, isLoading, error, pendingAction, refresh: refreshSnapshot,
    retrySlot, approveResults, cancel, replacePlan, resumePending };
}

function parseProjectionEvent(value: string): ImageAgentProjectionEvent | undefined {
  try {
    const parsed = JSON.parse(value) as Partial<ImageAgentProjectionEvent>;
    if (parsed.schema_version !== "image-agent.projection.v1" || typeof parsed.type !== "string" ||
        !Number.isSafeInteger(parsed.projection_version) || Number(parsed.projection_version) <= 0) return undefined;
    return parsed as ImageAgentProjectionEvent;
  } catch { return undefined; }
}

function cloneSlot(slot: ImageAgentSlot): ImageAgentSlot {
  return { ...slot, source_asset_ids: [...slot.source_asset_ids],
    style_reference_ids: slot.style_reference_ids ? [...slot.style_reference_ids] : undefined };
}
function newActionID() { return globalThis.crypto.randomUUID(); }
function isAuthenticationError(error: unknown) {
  return error instanceof ApiError && (error.status === 401 || error.status === 403);
}
function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Image Agent 请求失败";
}
