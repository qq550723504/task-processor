"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  approveImageAgentResults,
  cancelImageAgentRun,
  getImageAgentRun,
  imageAgentEventsUrl,
  replaceImageAgentPlan,
  retryImageAgentSlot,
} from "@/lib/api/image-agent";
import type {
  ImageAgentPlan,
  ImageAgentProjection,
  ImageAgentProjectionEvent,
  ImageAgentSlot,
} from "@/lib/types/image-agent";

const reconnectDelayMs = 50;

export function useImageAgentRun({
  runId,
  initialRun,
}: {
  runId: string;
  initialRun?: ImageAgentProjection;
}) {
  const validInitial = initialRun?.run.id === runId ? initialRun : undefined;
  const [projection, setProjection] = useState<ImageAgentProjection | undefined>(
    validInitial,
  );
  const [isLoading, setIsLoading] = useState(!validInitial);
  const [error, setError] = useState<string>();
  const [pendingAction, setPendingAction] = useState<string>();
  const projectionRef = useRef(validInitial);
  const cursorRef = useRef(validInitial?.last_event_id ?? 0);
  const versionRef = useRef(validInitial?.run.version ?? 0);
  const mountedRef = useRef(true);
  const requestSequenceRef = useRef(0);
  const committedRequestRef = useRef(0);
  const actionIDsRef = useRef(new Map<string, string>());

  const commitSnapshot = useCallback(
    (next: ImageAgentProjection, requestSequence = 0) => {
      if (!mountedRef.current || next.run.id !== runId) {
        return;
      }
      if (requestSequence > 0 && requestSequence < committedRequestRef.current) {
        return;
      }
      committedRequestRef.current = Math.max(
        committedRequestRef.current,
        requestSequence,
      );
      projectionRef.current = next;
      cursorRef.current = next.last_event_id;
      versionRef.current = next.run.version;
      setProjection(next);
      setIsLoading(false);
    },
    [runId],
  );

  const refreshSnapshot = useCallback(async () => {
    const requestSequence = ++requestSequenceRef.current;
    const next = await getImageAgentRun(runId);
    commitSnapshot(next, requestSequence);
    return next;
  }, [commitSnapshot, runId]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (initialRun?.run.id === runId) {
      commitSnapshot(initialRun);
    }
  }, [commitSnapshot, initialRun, runId]);

  useEffect(() => {
    let disposed = false;
    let source: EventSource | undefined;
    let projectionListener: EventListener | undefined;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let recovering = false;

    const closeSource = () => {
      if (source) {
        if (projectionListener) {
          source.removeEventListener("projection", projectionListener);
        }
        source.close();
        source = undefined;
        projectionListener = undefined;
      }
    };

    const scheduleRecovery = () => {
      if (disposed || reconnectTimer) {
        return;
      }
      reconnectTimer = setTimeout(() => {
        reconnectTimer = undefined;
        void recover();
      }, reconnectDelayMs);
    };

    const connect = () => {
      if (disposed || source) {
        return;
      }
      source = new EventSource(imageAgentEventsUrl(runId));
      projectionListener = (rawEvent: Event) => {
        const event = rawEvent as MessageEvent<string>;
        const cursor = Number.parseInt(event.lastEventId, 10);
        const envelope = parseProjectionEvent(event.data);
        if (!envelope || !Number.isSafeInteger(cursor) || cursor <= 0) {
          return;
        }
        if (
          cursor <= cursorRef.current ||
          envelope.projection_version <= versionRef.current
        ) {
          return;
        }
        cursorRef.current = cursor;
        versionRef.current = envelope.projection_version;
        void recover();
      };
      source.addEventListener("projection", projectionListener);
      source.onerror = () => {
        void recover();
      };
    };

    async function recover() {
      if (disposed || recovering) {
        return;
      }
      recovering = true;
      closeSource();
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = undefined;
      }
      try {
        await refreshSnapshot();
        if (!disposed) {
          setError(undefined);
          reconnectTimer = setTimeout(() => {
            reconnectTimer = undefined;
            connect();
          }, reconnectDelayMs);
        }
      } catch (cause) {
        if (!disposed) {
          setIsLoading(false);
          setError(errorMessage(cause));
          scheduleRecovery();
        }
      } finally {
        recovering = false;
      }
    }

    if (projectionRef.current?.run.id === runId) {
      connect();
    } else {
      void recover();
    }

    return () => {
      disposed = true;
      closeSource();
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
      }
    };
  }, [refreshSnapshot, runId]);

  const executeCommand = useCallback(
    async (
      key: string,
      execute: (actionID: string) => Promise<void>,
    ) => {
      const actionID = actionIDsRef.current.get(key) ?? newActionID();
      actionIDsRef.current.set(key, actionID);
      setPendingAction(key);
      setError(undefined);
      try {
        await execute(actionID);
      } catch (cause) {
        if (mountedRef.current) {
          setError(errorMessage(cause));
          setPendingAction(undefined);
        }
        return;
      }
      actionIDsRef.current.delete(key);
      try {
        await refreshSnapshot();
      } catch (cause) {
        if (mountedRef.current) {
          setError(`命令已接受，但刷新状态失败：${errorMessage(cause)}`);
        }
      } finally {
        if (mountedRef.current) {
          setPendingAction(undefined);
        }
      }
    },
    [refreshSnapshot],
  );

  const retrySlot = useCallback(
    async (slotID: string) => {
      const current = projectionRef.current;
      if (!current) return;
      const revision = current.plan.revision;
      await executeCommand(`retry:${revision}:${slotID}`, (actionID) =>
        retryImageAgentSlot(runId, slotID, revision, actionID),
      );
    },
    [executeCommand, runId],
  );

  const approveResults = useCallback(async () => {
    const current = projectionRef.current;
    if (!current?.result_digest) return;
    const revision = current.plan.revision;
    const digest = current.result_digest;
    await executeCommand(`approve:${revision}:${digest}`, (actionID) =>
      approveImageAgentResults(runId, revision, digest, actionID),
    );
  }, [executeCommand, runId]);

  const cancel = useCallback(async () => {
    const current = projectionRef.current;
    if (!current) return;
    const revision = current.plan.revision;
    await executeCommand(`cancel:${revision}`, (actionID) =>
      cancelImageAgentRun(runId, revision, actionID),
    );
  }, [executeCommand, runId]);

  const replacePlan = useCallback(
    async (slots: ImageAgentSlot[]) => {
      const current = projectionRef.current;
      if (!current) return;
      const expectedRevision = current.plan.revision;
      const draftKey = JSON.stringify(
        slots.map((slot) => [
          slot.id,
          slot.source_asset_ids,
          slot.style_reference_ids,
          slot.brief,
        ]),
      );
      await executeCommand(
        `replace:${expectedRevision}:${draftKey}`,
        (actionID) => {
          const plan: ImageAgentPlan = {
            ...current.plan,
            revision: expectedRevision + 1,
            parent_revision: expectedRevision,
            idempotency_key: `image-agent-plan-${actionID}`,
            slots: slots.map(cloneSlot),
          };
          return replaceImageAgentPlan(
            runId,
            expectedRevision,
            plan,
            actionID,
          );
        },
      );
    },
    [executeCommand, runId],
  );

  return {
    projection,
    isLoading,
    error,
    pendingAction,
    refresh: refreshSnapshot,
    retrySlot,
    approveResults,
    cancel,
    replacePlan,
  };
}

function parseProjectionEvent(value: string): ImageAgentProjectionEvent | undefined {
  try {
    const parsed = JSON.parse(value) as Partial<ImageAgentProjectionEvent>;
    if (
      parsed.schema_version !== "image-agent.projection.v1" ||
      typeof parsed.type !== "string" ||
      !Number.isSafeInteger(parsed.projection_version) ||
      Number(parsed.projection_version) <= 0
    ) {
      return undefined;
    }
    return parsed as ImageAgentProjectionEvent;
  } catch {
    return undefined;
  }
}

function cloneSlot(slot: ImageAgentSlot): ImageAgentSlot {
  return {
    ...slot,
    source_asset_ids: [...slot.source_asset_ids],
    style_reference_ids: slot.style_reference_ids
      ? [...slot.style_reference_ids]
      : undefined,
  };
}

function newActionID() {
  return globalThis.crypto.randomUUID();
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Image Agent 请求失败";
}
