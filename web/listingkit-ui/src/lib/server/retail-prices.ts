import "server-only";
import { parseRetailPriceCatalog, type RetailPriceCatalog } from "@/lib/api/retail-prices";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";

// Server-side fixed-path read. Never forwards identity, org, or cookie headers.
export async function readRetailPrices(): Promise<RetailPriceCatalog | null> {
  const raw = process.env.COMMERCIAL_API_ORIGIN;
  if (!raw) return null;
  let origin: string;
  try {
    const url = new URL(raw);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || (raw !== url.origin && raw !== `${url.origin}/`)) return null;
    origin = url.origin;
  } catch { return null; }
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5_000);
  try {
    const response = await fetch(`${origin}/api/v1/commercial/resource-offers`, {
      method: "GET", headers: { Accept: "application/json" }, cache: "no-store", redirect: "manual", signal: controller.signal,
    });
    if (response.status !== 200) { await response.body?.cancel(); return null; }
    const payload = await readBoundedStrictJSON(response, 32 * 1024, controller.signal);
    controller.signal.throwIfAborted();
    return parseRetailPriceCatalog(payload);
  } catch { return null; }
  finally { clearTimeout(timeout); }
}
