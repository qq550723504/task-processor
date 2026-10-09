// Same bounded streaming pattern as the existing ecosystem file proxy. Shared
// composition files stay with their Writer; no bytes reach disk during proxying.
export async function readToolPackage(
  response: Response,
  signal: AbortSignal,
): Promise<Uint8Array<ArrayBuffer>> {
  const reader = response.body?.getReader();
  if (!reader) throw new Error("missing package");
  const chunks: Uint8Array[] = [];
  let size = 0;
  const cancel = () => {
    void reader.cancel().catch(() => undefined);
  };
  signal.addEventListener("abort", cancel, { once: true });
  try {
    while (true) {
      signal.throwIfAborted();
      const v = await reader.read();
      signal.throwIfAborted();
      if (v.done) break;
      size += v.value.length;
      if (size > 2 << 20) throw new Error("package exceeds bound");
      chunks.push(v.value);
    }
    const out = new Uint8Array(size);
    let at = 0;
    for (const v of chunks) {
      out.set(v, at);
      at += v.length;
    }
    return out;
  } catch (e) {
    cancel();
    throw e;
  } finally {
    signal.removeEventListener("abort", cancel);
    reader.releaseLock();
  }
}
