import { parseTree, type Node, type ParseError } from "jsonc-parser";

export class InvalidStrictJSONResponseError extends Error {}

/** Reads and parses an application/json response using an actual byte limit. */
export async function readBoundedStrictJSON(response: Response, maxBytes: number, signal?: AbortSignal): Promise<unknown> {
  if (!/^application\/json(?:\s*;|$)/i.test(response.headers.get("content-type") ?? "")) {
    void response.body?.cancel().catch(() => undefined);
    throw new InvalidStrictJSONResponseError("Invalid JSON content type");
  }
  const reader = response.body?.getReader();
  if (!reader) throw new InvalidStrictJSONResponseError("Missing JSON body");
  const cancel = () => { void reader.cancel().catch(() => undefined); };
  signal?.addEventListener("abort", cancel, { once: true });
  try {
    const chunks: Uint8Array[] = [];
    let size = 0;
    while (true) {
      signal?.throwIfAborted();
      const { done, value } = await reader.read();
      signal?.throwIfAborted();
      if (done) break;
      size += value.byteLength;
      if (size > maxBytes) throw new InvalidStrictJSONResponseError("JSON response too large");
      chunks.push(value);
    }
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    return parseStrictJSON(bytes);
  } catch (error) {
    cancel();
    throw error;
  } finally {
    signal?.removeEventListener("abort", cancel);
    reader.releaseLock();
  }
}

function parseStrictJSON(bytes: Uint8Array) {
  try {
    const raw = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    const errors: ParseError[] = [];
    const root = parseTree(raw, errors, { disallowComments: true, allowTrailingComma: false });
    if (!root || errors.length) throw new Error("Invalid JSON");
    const pending: Node[] = [root];
    while (pending.length) {
      const node = pending.pop()!;
      if (node.type === "object") {
        const keys = new Set<string>();
        for (const property of node.children ?? []) {
          const key = property.children?.[0]?.value;
          if (keys.has(key)) throw new Error("Duplicate JSON field");
          keys.add(key);
        }
      }
      for (const child of node.children ?? []) pending.push(child);
    }
    return JSON.parse(raw);
  } catch {
    throw new InvalidStrictJSONResponseError("Invalid JSON");
  }
}
