const sheinStudioSaveQueues = new Map<string, Promise<void>>();

export function enqueueSheinStudioSave<T>(
  queueKey: string,
  operation: () => Promise<T>,
): Promise<T> {
  const key = queueKey.trim() || "default";
  const existingTail = sheinStudioSaveQueues.get(key);
  const run = existingTail
    ? existingTail.catch(() => undefined).then(operation)
    : operation();
  const nextTail = run.then(
    () => undefined,
    () => undefined,
  );
  sheinStudioSaveQueues.set(key, nextTail);
  return run.finally(() => {
    if (sheinStudioSaveQueues.get(key) === nextTail) {
      sheinStudioSaveQueues.delete(key);
    }
  });
}

export function resetSheinStudioSaveQueueForTest() {
  sheinStudioSaveQueues.clear();
}
