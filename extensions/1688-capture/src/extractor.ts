import { parseTree, type Node, type ParseError } from 'jsonc-parser';
import type { CapturePayload, Evidence, Price } from './wire';
import { byteLength, fail, MAX_BYTES, pageSource, validateCapture } from './validation';

// EXTRACT: known 1688 productTitle/gallery/Root field semantics only. No legacy
// service, window.context access, profile, network or arbitrary script execution.
export async function captureDocument(doc: Document, source: string, now: Date): Promise<CapturePayload> {
  const started = performance.now();
  const identity = pageSource(source);
  if (pageSource(doc.URL).sourceURL !== identity.sourceURL) fail('PAGE_CHANGED');
  let raw: string | undefined;
  const scripts = doc.querySelectorAll('script:not([src])');
  if (scripts.length > 256) fail('CAPTURE_TOO_LARGE');
  for (const script of scripts) {
    // Inspect only a bounded prefix; unrelated scripts are never copied.
    const first = script.firstChild;
    if (!first || first.nodeType !== 3) continue;
    const text = first as Text;
    const prefix = text.substringData(0, 80);
    const match = /^\s*(?:window\.)?context\s*=\s*/.exec(prefix);
    if (!match) continue;
    if (raw !== undefined) fail('UNSUPPORTED_PAGE');
    if (text.length > MAX_BYTES) fail('CAPTURE_TOO_LARGE');
    const candidate = text.data.slice(match[0].length).trim().replace(/;\s*$/, '');
    if (byteLength(candidate) > MAX_BYTES) fail('CAPTURE_TOO_LARGE');
    raw = candidate;
  }
  if (raw === undefined) fail('UNSUPPORTED_PAGE');
  const errors: ParseError[] = [];
  const root = parseTree(raw, errors, { disallowComments: true, allowTrailingComma: false });
  if (!root || errors.length) fail('UNSUPPORTED_PAGE');
  let nodes = 0;
  function inspect(node: Node, depth = 0) {
    if (++nodes > 16384 || depth > 32 || performance.now() - started > 8000) fail('CAPTURE_TOO_LARGE');
    if (node.type === 'object') {
      const keys = node.children?.map(n => n.children?.[0].value) ?? [];
      if (new Set(keys).size !== keys.length) fail('UNSUPPORTED_PAGE');
    }
    for (const child of node.children ?? []) inspect(child, depth + 1);
  }
  inspect(root);
  function path(node: Node | undefined, ...keys: string[]): Node | undefined {
    for (const key of keys) {
      if (!node) return undefined;
      if (node.type !== 'object') fail('UNSUPPORTED_PAGE');
      node = node.children?.find(p => p.children?.[0].value === key)?.children?.[1];
    }
    return node;
  }
  function string(node: Node | undefined, numeric = false): string | null {
    if (!node || node.type === 'null') return null;
    if (node.type === 'string') return node.value as string;
    if (numeric && node.type === 'number') return raw!.slice(node.offset, node.offset + node.length);
    return fail('UNSUPPORTED_PAGE');
  }
  function array(node: Node | undefined): Node[] {
    if (!node) return [];
    if (node.type !== 'array' || (node.children?.length ?? 0) > 256) fail('UNSUPPORTED_PAGE');
    return node.children ?? [];
  }
  const data = path(root, 'result', 'data');
  const model = path(data, 'Root', 'fields', 'dataJson');
  if (string(path(model, 'tempModel', 'offerId'), true) !== identity.offerID) fail('PAGE_CHANGED');
  const title = string(path(data, 'productTitle', 'fields', 'title'));
  if (!title?.trim() || /验证码|请登录|访问受限|captcha|access denied/i.test(title)) fail('UNSUPPORTED_PAGE');
  const e: Evidence = {
    schemaVersion: 1, ...identity, title, description: null, attributes: [], variants: [], priceFacts: [], images: [],
    capturedAt: now.toISOString(), contentSHA256: '0'.repeat(64), parserVersion: '1688-browser-dom/v1', warnings: [], missingFacts: [],
  };
  const missing = (field: string) => {
    e.missingFacts.push({ field, reason: 'not_observed' });
    e.warnings.push({ code: 'MISSING_FACT', field });
  };
  e.attributes = array(path(root, 'result', 'global', 'globalData', 'model', 'offerDetail', 'featureAttributes')).map(n => ({
    name: string(path(n, 'name')) ?? fail('UNSUPPORTED_PAGE'), value: string(path(n, 'value')) ?? fail('UNSUPPORTED_PAGE'),
  }));
  e.images = array(path(data, 'gallery', 'fields', 'offerImgList')).map(n => ({ url: string(n) ?? fail('UNSUPPORTED_PAGE'), role: 'source' }));
  const price = (node: Node, field: string, amountKey = 'price'): Price | null => {
    const amount = string(path(node, amountKey), true);
    if (amount === null) { missing(field); return null; }
    const currency = string(path(node, 'currency'));
    if (currency === null) missing(`${field}.currency`);
    const minQuantity = string(path(node, 'beginAmount'), true);
    return { amount, currency, ...(minQuantity !== null ? { minQuantity } : {}) };
  };
  for (const item of array(path(data, 'price', 'fields', 'finalPriceModel', 'tradeWithoutPromotion', 'offerPriceRanges'))) {
    const value = price(item, `priceFacts[${e.priceFacts.length}]`);
    if (value) e.priceFacts.push(value);
  }
  const sku = path(model, 'skuModel');
  const props = array(path(sku, 'skuProps')).map(n => string(path(n, 'prop')) ?? fail('UNSUPPORTED_PAGE'));
  const info = path(sku, 'skuInfoMap');
  if (info && (info.type !== 'object' || (info.children?.length ?? 0) > 256)) fail('UNSUPPORTED_PAGE');
  for (const pair of info?.children ?? []) {
    const key = pair.children![0].value as string; const item = pair.children![1];
    const id = string(path(item, 'skuId'), true);
    // skuId identifies the provider variant; it does not establish a seller SKU.
    missing(`variants[${e.variants.length}].sku`);
    const values = key.split('&gt;');
    if (values.length !== props.length) fail('UNSUPPORTED_PAGE');
    e.variants.push({ sourceID: id, sku: null, title: null,
      attributes: props.map((name, i) => ({ name, value: values[i] })), price: price(item, `variants[${e.variants.length}].price`) });
  }
  // Only explicitly identified, already rendered description text. Never frames
  // or external detailUrl fetches, and never whole-body text/HTML.
  const description = doc.querySelector('#desc-lazyload-container');
  if (description && !description.querySelector('script,iframe,input,form')) {
    const texts = doc.createTreeWalker(description, 4); const parts: string[] = []; let length = 0;
    for (let n = texts.nextNode(); n; n = texts.nextNode()) {
      length += (n as Text).length;
      if (length > 8192) fail('CAPTURE_TOO_LARGE');
      parts.push((n as Text).data);
    }
    e.description = parts.join('').trim() || null;
  }
  for (const field of ['description', 'attributes', 'variants', 'priceFacts', 'images'] as const) {
    if (!e[field] || e[field].length === 0) missing(field);
  }
  const payload = validateCapture({ captureVersion: 1, evidence: e });
  const projection = JSON.stringify({ sourceURL: e.sourceURL, offerID: e.offerID, title: e.title, description: e.description,
    attributes: e.attributes, variants: e.variants, priceFacts: e.priceFacts, images: e.images });
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(projection));
  payload.evidence.contentSHA256 = Array.from(new Uint8Array(digest), b => b.toString(16).padStart(2, '0')).join('');
  if (performance.now() - started > 8000) fail('CAPTURE_TIMEOUT');
  if (pageSource(doc.URL).sourceURL !== identity.sourceURL) fail('PAGE_CHANGED');
  return payload;
}
