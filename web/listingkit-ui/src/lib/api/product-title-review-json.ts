import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

export async function readProductReviewJSON(response: Response, maxBytes: number, signal?: AbortSignal): Promise<unknown> {
  const payload = await readBoundedStrictJSON(response, maxBytes, signal);
  const pending: unknown[] = [payload];
  while (pending.length) {
    const value = pending.pop();
    if (typeof value === "string" && /[\uD800-\uDFFF]/u.test(value)) {
      throw new InvalidStrictJSONResponseError("Invalid JSON Unicode");
    }
    if (Array.isArray(value)) pending.push(...value);
    else if (value && typeof value === "object") {
      for (const [key, entry] of Object.entries(value)) pending.push(key, entry);
    }
  }
  signal?.throwIfAborted();
  return payload;
}
