import { useEffect, useRef } from "react";

import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  type DedicatedBatchLocalSnapshot,
  resolveDedicatedBatchHydration,
} from "@/lib/shein-studio/batch-hydration";

type InitialBatchHydrationParams = {
  initialBatchId?: string;
  getHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  loadLocalSnapshot: () => DedicatedBatchLocalSnapshot | null;
  loadHydratedBatch: (batch: SheinStudioWorkbenchHydratedBatch) => void;
  promptOverride?: string;
  resolveHydration?: typeof resolveDedicatedBatchHydration;
  setGenerationError: (message: string) => void;
  setLoaded: (loaded: boolean) => void;
  setQueueMessage: (message: string) => void;
};

export function useSheinStudioInitialBatchHydration({
  initialBatchId,
  getHydratedBatch,
  loadLocalSnapshot,
  loadHydratedBatch,
  promptOverride,
  resolveHydration = resolveDedicatedBatchHydration,
  setGenerationError,
  setLoaded,
  setQueueMessage,
}: InitialBatchHydrationParams) {
  const loadedInitialBatchIdRef = useRef<string | null>(null);
  const loadHydratedBatchRef = useRef(loadHydratedBatch);

  useEffect(() => {
    loadHydratedBatchRef.current = loadHydratedBatch;
  }, [loadHydratedBatch]);

  useEffect(() => {
    if (!initialBatchId) {
      loadedInitialBatchIdRef.current = null;
      return;
    }
    const batchId = initialBatchId;
    if (loadedInitialBatchIdRef.current === batchId) {
      return;
    }

    let cancelled = false;

    async function loadInitialBatch() {
      try {
        const hydratedBatch = await getHydratedBatch(batchId);
        if (cancelled || !hydratedBatch) {
          return;
        }
        const batchWithLocalSnapshot = resolveHydration({
          batchId,
          hydratedBatch,
          localSnapshot: loadLocalSnapshot(),
          promptOverride,
        });
        loadedInitialBatchIdRef.current = batchId;
        loadHydratedBatchRef.current(batchWithLocalSnapshot);
        setLoaded(true);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const message = `当前批次加载失败：${
          error instanceof Error ? error.message : "未知错误"
        }。请重新登录后再继续。`;
        setGenerationError(message);
        setQueueMessage(message);
        setLoaded(true);
      }
    }

    void loadInitialBatch();

    return () => {
      cancelled = true;
    };
  }, [
    getHydratedBatch,
    initialBatchId,
    loadLocalSnapshot,
    promptOverride,
    resolveHydration,
    setGenerationError,
    setLoaded,
    setQueueMessage,
  ]);
}
