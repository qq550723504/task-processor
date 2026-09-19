import { z } from 'zod';
import type { CapturePayload } from './wire';

export const MAX_BYTES = 2 * 1024 * 1024;
export const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const utf8 = new TextEncoder();
export const byteLength = (value: string) => utf8.encode(value).length;
export function fail(code: string): never { throw new Error(code); }

// Client-side URL admission only. The sourcing owner derives trusted identity.
export function pageSource(input: string) {
  if (input.length > 2048 || byteLength(input) > 2048 || /[\\\u0000-\u0020]/.test(input) || /%(?![0-9a-f]{2})/i.test(input)) fail('INVALID_PAGE');
  const match = /^https?:\/\/detail\.1688\.com(?::(80|443))?\/offer\/([1-9][0-9]{0,19})\.html(?:[?#].*)?$/i.exec(input);
  if (!match) fail('INVALID_PAGE');
  const url = new URL(input);
  if (url.port) fail('INVALID_PAGE');
  return { sourceURL: `https://detail.1688.com/offer/${match[2]}.html`, offerID: match[2] };
}

const text = z.string().max(8192).refine(v => byteLength(v) <= 8192 && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f\ud800-\udfff]/u.test(v));
const decimal = text.regex(/^(0|[1-9][0-9]{0,15})(\.[0-9]{1,8})?$/);
const sensitiveName = /(?:cookie|authorization|password|passwd|token|session|profile|localstorage|sessionstorage)/i;
const attribute = z.strictObject({ name: text.min(1).refine(v => !sensitiveName.test(v)), value: text });
const price = z.strictObject({ amount: decimal, currency: text.regex(/^[A-Z]{3}$/).nullable(), minQuantity: decimal.nullable().optional() });
const attrs = z.array(attribute).max(256).refine(a => new Set(a.map(v => v.name)).size === a.length);
export function publicImage(value: string) {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && !url.username && !url.password && !url.port && !url.search && !url.hash
      && /^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/i.test(url.hostname)
      && !/(?:localhost|\.local|\.internal|\.test|\.invalid|\.localhost|\.example)$/i.test(url.hostname)
      && !sensitiveName.test(url.pathname) && !/%(?:00|0a|0d)/i.test(value);
  } catch { return false; }
}
const evidence = z.strictObject({
  schemaVersion: z.literal(1), sourceURL: text, offerID: text.regex(/^[1-9][0-9]{0,19}$/),
  title: text.min(1).nullable(), description: text.nullable(), attributes: attrs,
  variants: z.array(z.strictObject({ sourceID: text.nullable(), sku: text.nullable(), title: text.nullable(), attributes: attrs, price: price.nullable() })).max(256),
  priceFacts: z.array(price).max(256), images: z.array(z.strictObject({ url: text.refine(publicImage), role: text })).max(256),
  capturedAt: text.refine(v => /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(v) && Number.isFinite(Date.parse(v))),
  contentSHA256: text.regex(/^[a-f0-9]{64}$/), parserVersion: z.literal('1688-browser-dom/v1'),
  warnings: z.array(z.strictObject({ code: text, field: text })).max(256),
  missingFacts: z.array(z.strictObject({ field: text, reason: text })).max(256),
});
const schema = z.strictObject({ captureVersion: z.literal(1), evidence });
export function validateCapture(value: unknown): CapturePayload {
  const parsed = schema.safeParse(value);
  if (!parsed.success) fail('INVALID_CAPTURE');
  const payload = parsed.data;
  const e = payload.evidence;
  if (!e.title?.trim() || pageSource(e.sourceURL).offerID !== e.offerID || pageSource(e.sourceURL).sourceURL !== e.sourceURL) fail('INVALID_CAPTURE');
  let items = e.attributes.length + e.variants.length + e.priceFacts.length + e.images.length + e.warnings.length + e.missingFacts.length;
  const ids = new Set<string>(); const skus = new Set<string>();
  for (const v of e.variants) {
    items += v.attributes.length;
    if ((v.sourceID && ids.has(v.sourceID)) || (v.sku && skus.has(v.sku))) fail('INVALID_CAPTURE');
    if (v.sourceID) ids.add(v.sourceID); if (v.sku) skus.add(v.sku);
  }
  if (items > 1024 || byteLength(JSON.stringify(payload)) > MAX_BYTES) fail('CAPTURE_TOO_LARGE');
  return payload;
}

export function appURL(input: string, fixture = false) {
  const url = new URL(input);
  if (url.username || url.password || url.search || url.hash || url.pathname !== '/capture/1688') fail('INVALID_APP_URL');
  if (fixture ? url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port : url.protocol !== 'https:' || !publicImage(`${url.origin}/capture`)) fail('INVALID_APP_URL');
  return url.href;
}
