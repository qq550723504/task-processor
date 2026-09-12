import { z } from "zod";
import { canonical1688Source, isAcquisitionUUID } from "./product-acquisition";

export const BROWSER_CAPTURE_MAX_BYTES = 2 * 1024 * 1024;
export const BROWSER_CAPTURE_BASE = "/api/workbench/sourcing/1688/browser-captures";
function wellFormed(value: string) {
  for (let i = 0; i < value.length; i++) {
    const unit = value.charCodeAt(i);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(++i);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) return false;
  }
  return new TextEncoder().encode(value).byteLength <= 8192;
}
const text = z.string().refine(wellFormed);
const attribute = z.object({ name: text, value: text }).strict();
const price = z.object({ amount: text, currency: text.nullable(), minQuantity: text.nullable().optional() }).strict();
const evidence = z.object({
  schemaVersion: z.literal(1), sourceURL: text, offerID: text,
  title: text.nullable(), description: text.nullable(),
  attributes: z.array(attribute).max(256),
  variants: z.array(z.object({ sourceID: text.nullable(), sku: text.nullable(), title: text.nullable(), attributes: z.array(attribute).max(256), price: price.nullable() }).strict()).max(256),
  priceFacts: z.array(price).max(256),
  images: z.array(z.object({ url: text, role: text }).strict()).max(256),
  capturedAt: text.refine((v) => /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(v) && Number.isFinite(Date.parse(v))),
  contentSHA256: z.string().regex(/^[0-9a-f]{64}$/), parserVersion: z.literal("1688-browser-dom/v1"),
  warnings: z.array(z.object({ code: text, field: text }).strict()).max(256),
  missingFacts: z.array(z.object({ field: text, reason: text }).strict()).max(256),
}).strict();
// Transport only: Go owns canonicalization, digest and business mapping.
export const browserCaptureSchema = z.object({ captureVersion: z.literal(1), evidence }).strict().refine(({ evidence: e }) => {
  const source = canonical1688Source(e.sourceURL);
  const count = e.attributes.length + e.variants.length + e.variants.reduce((n, v) => n + v.attributes.length, 0) + e.priceFacts.length + e.images.length + e.warnings.length + e.missingFacts.length;
  return source === `https://detail.1688.com/offer/${e.offerID}.html` && count <= 1024;
}).refine((v) => new TextEncoder().encode(JSON.stringify(v)).byteLength <= BROWSER_CAPTURE_MAX_BYTES);
export type BrowserCapturePayload = z.infer<typeof browserCaptureSchema>;
export function isBrowserCapturePath(method: string, path: string[]) {
  if (path[0] !== "sourcing" || path[1] !== "1688" || path[2] !== "browser-captures") return false;
  if (method === "POST") return path.length === 3 || (path.length === 4 && path[3] === "verify");
  return method === "GET" && ((path.length === 4 && isAcquisitionUUID(path[3]!)) || (path.length === 5 && path[3] === "by-key" && isAcquisitionUUID(path[4]!)));
}
export function isBrowserCaptureRequestURL(method: string, rawURL: string) {
  try {
    const url = new URL(rawURL);
    if (!url.pathname.startsWith("/api/workbench/")) return false;
    return isBrowserCapturePath(method, url.pathname.slice("/api/workbench/".length).split("/"));
  } catch { return false; }
}
