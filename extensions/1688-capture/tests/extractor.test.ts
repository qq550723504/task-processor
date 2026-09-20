import { describe, expect, it } from 'vitest';
import { JSDOM } from 'jsdom';
import { readFileSync } from 'node:fs';
import { captureDocument } from '../src/extractor';

const source = 'https://detail.1688.com/offer/981645030344.html';
// Observed 1688 wrapper: the document is the single object-literal argument of an
// IIFE call, assigned after an unrelated window.contextPath statement.
const iife = (payload: string) => '(function(b,d){var c=d.module||{};var e={};for(var a in c){if(typeof c[a]==="string"){e[a]=c[a]}}'
  + `Object.assign(e,c[b]||{});Object.assign(d,{module:e});return d})(window.contextPath,${payload})`;
const observedShape = (payload: string) => new JSDOM(`<script>\nwindow.contextPath = "/default";\nwindow.context = ${iife(payload)};\n</script>`, { url: source });
const model = { result: { data: {
    productTitle: { fields: { title: '公开商品 <img onerror=alert(1)>' } },
    gallery: { fields: { offerImgList: ['https://cbu01.alicdn.com/img/ibank/product.jpg'] } },
    Root: { fields: { dataJson: { tempModel: { offerId: '981645030344' }, skuModel: {
      skuProps: [{ prop: '颜色' }], skuInfoMap: { '蓝色': { skuId: '99999999999999999999', price: '1.23000001' } },
    } } } },
    price: { fields: { finalPriceModel: { tradeWithoutPromotion: {
      offerPriceRanges: [{ price: '12.34000001', beginAmount: '2' }],
    } } } },
  }, global: { globalData: { model: { offerDetail: { featureAttributes: [{ name: '材质', value: '棉' }] } } } } } };
function fixture(extra = '') {
  return new JSDOM(`<script>window.context = ${JSON.stringify(model)};</script>${extra}`, { url: source });
}

describe('explicit current-page capture', () => {
  it('matches the shared browser wire golden without a second sourcing mapper', async () => {
    const html=readFileSync('tests/fixtures/product.html','utf8');
    const golden=JSON.parse(readFileSync('tests/fixtures/capture-v1.json','utf8'));
    const doc=new JSDOM(html,{url:source}).window.document;
    expect(await captureDocument(doc,source,new Date('2026-09-12T00:00:00Z'))).toEqual(golden);
  });
  it('projects actual facts, exact decimals and missing currency into v1 evidence', async () => {
    const dom = fixture('<div id="desc-lazyload-container">实际商品描述</div>');
    const payload = await captureDocument(dom.window.document, source, new Date('2026-09-12T00:00:00Z'));
    expect(payload.captureVersion).toBe(1);
    expect(payload.evidence).toMatchObject({ schemaVersion: 1, sourceURL: source, offerID: '981645030344',
      parserVersion: '1688-browser-dom/v2', title: '公开商品 <img onerror=alert(1)>',
      attributes: [{ name: '材质', value: '棉' }],
      variants: [{ sourceID: '99999999999999999999', price: { amount: '1.23000001', currency: null } }],
      priceFacts: [{ amount: '12.34000001', currency: null, minQuantity: '2' }],
      images: [{ url: 'https://cbu01.alicdn.com/img/ibank/product.jpg', role: 'source' }],
    });
    expect(payload.evidence.contentSHA256).toMatch(/^[a-f0-9]{64}$/);
    expect(payload.evidence.missingFacts.some(f => f.field.includes('currency'))).toBe(true);
  });

  it('captures the observed 1688 shape where the payload is an IIFE argument', async () => {
    const html = readFileSync('tests/fixtures/product-real-shape.html', 'utf8');
    const golden = JSON.parse(readFileSync('tests/fixtures/capture-v1.json', 'utf8'));
    const doc = new JSDOM(html, { url: source }).window.document;
    expect(await captureDocument(doc, source, new Date('2026-09-12T00:00:00Z'))).toEqual(golden);
  });

  it('produces identical evidence from the observed shape and the JSON-literal shape', async () => {
    const payload = JSON.stringify(model);
    const at = new Date('2026-09-12T00:00:00Z');
    const observed = await captureDocument(observedShape(payload).window.document, source, at);
    const literal = await captureDocument(fixture().window.document, source, at);
    expect(observed).toEqual(literal);
  });

  it('rejects ambiguous or non-payload context assignments instead of guessing', async () => {
    const payload = JSON.stringify(model);
    const bodies = [
      `window.contextPath = "/default";`,
      'window.context = (function(b,d){return d})',
      `window.context = (function(a,b,c){return c})(window.contextPath,${payload},{"extra":1})`,
      'window.context = (function(a,b){return b})(window.contextPath,window.contextPath)',
    ];
    for (const body of bodies) {
      const doc = new JSDOM(`<script>${body}</script>`, { url: source }).window.document;
      await expect(captureDocument(doc, source, new Date())).rejects.toThrow();
    }
    const duplicated = fixture(`<script>window.context = ${payload};</script>`);
    await expect(captureDocument(duplicated.window.document, source, new Date())).rejects.toThrow('UNSUPPORTED_PAGE');
  });

  it('reads price ranges from any result.data key, as the sourcing owner does', async () => {
    const dom = fixture(); const node = dom.window.document.querySelector('script')!;
    node.textContent = node.textContent!.replace('"price":{', '"priceInfo":{');
    const payload = await captureDocument(dom.window.document, source, new Date());
    expect(payload.evidence.priceFacts).toEqual([{ amount: '12.34000001', currency: null, minQuantity: '2' }]);
  });

  // The live page emits "skuWeight":{6290953586037:0.3}: object keys are bare
  // decimal numbers, which strict JSON forbids. Only that key position is
  // normalized; every other strictness rule still applies.
  it('reads the live page shape whose object keys are bare decimal numbers', async () => {
    const doc = new JSDOM(readFileSync('tests/fixtures/product.html', 'utf8'), { url: source }).window.document;
    const payload = await captureDocument(doc, source, new Date('2026-09-12T00:00:00Z'));
    expect(payload.evidence.title).toBe('棉质收纳袋');
    expect(payload.evidence.variants).toHaveLength(1);
  });

  it('normalizes only object key positions, never string contents', async () => {
    const text = '裸键形字符串 {123:x} 与逗号, 保留';
    const bulk = JSON.parse(JSON.stringify(model));
    bulk.result.data.productTitle.fields.title = text;
    const payload = await captureDocument(observedShape(JSON.stringify(bulk)).window.document, source, new Date());
    expect(payload.evidence.title).toBe(text);
  });

  it('still rejects comments, trailing commas and duplicate keys after normalization', async () => {
    const mutations: Array<[string, (raw: string) => string]> = [
      ['duplicate key', raw => raw.replace('"title":', '"title":"shadow","title":')],
      ['trailing comma', raw => `${raw.slice(0, -1)},}`],
      ['comment', raw => raw.replace('"title":', '/*c*/"title":')],
    ];
    for (const [name, mutate] of mutations) {
      const dom = fixture(); const node = dom.window.document.querySelector('script')!;
      const mutated = mutate(node.textContent!);
      expect(mutated, `${name} mutation must change the payload`).not.toBe(node.textContent);
      node.textContent = mutated;
      await expect(captureDocument(dom.window.document, source, new Date()), name).rejects.toThrow('UNSUPPORTED_PAGE');
    }
  });

  it('accepts live-scale node counts and still bounds the inspected tree', async () => {
    const at = new Date('2026-09-12T00:00:00Z');
    const build = (keys: number) => {
      const grow = JSON.parse(JSON.stringify(model)) as typeof model & { result: { data: Record<string, unknown> } };
      const bulk: Record<string, number> = {};
      for (let i = 0; i < keys; i++) bulk[`k${i}`] = i;
      grow.result.data.bulk = bulk;
      return observedShape(JSON.stringify(grow)).window.document;
    };
    // The live page needs ~16.5k nodes, above the previous 16384 bound.
    await expect(captureDocument(build(7000), source, at)).resolves.toBeTruthy();
    await expect(captureDocument(build(30000), source, at)).rejects.toThrow('CAPTURE_TOO_LARGE');
  });

  it('keeps the real-shape fixture a byte-exact wrapping of the plain fixture', async () => {
    const plain = readFileSync('tests/fixtures/product.html', 'utf8');
    const wrapped = readFileSync('tests/fixtures/product-real-shape.html', 'utf8');
    const start = plain.indexOf('window.context = ') + 'window.context = '.length;
    const payload = plain.slice(start, plain.indexOf(';</script>', start));
    expect(payload.startsWith('{')).toBe(true);
    expect(wrapped).toContain(`window.contextPath = "/default";`);
    expect(wrapped).toContain(`})(window.contextPath,${payload});`);
  });

  it('keeps skuId as source identity without inventing a separately observed SKU', async () => {    const payload=await captureDocument(fixture().window.document,source,new Date());
    expect(payload.evidence.variants[0].sourceID).toBe('99999999999999999999');
    expect(payload.evidence.variants[0].sku).toBeNull();
    expect(payload.evidence.missingFacts).toContainEqual({field:'variants[0].sku',reason:'not_observed'});
  });

  it('never touches sensitive getters, global objects or unrelated input/script data', async () => {
    const dom = fixture('<input type="password" value="PASSWORD_CANARY"><script>window.secret="AUTH_CANARY"</script>');
    for (const name of ['cookie']) Object.defineProperty(dom.window.document, name, { get() { throw Error('sensitive access'); } });
    for (const name of ['localStorage', 'sessionStorage', 'context', 'history']) Object.defineProperty(dom.window, name, { get() { throw Error('sensitive access'); } });
    const payload = await captureDocument(dom.window.document, source, new Date());
    expect(JSON.stringify(payload)).not.toMatch(/PASSWORD_CANARY|AUTH_CANARY|localStorage|sessionStorage/);
  });

  it.each(['https://detail.1688.com.evil.test/offer/981645030344.html',
    'https://user:password@detail.1688.com/offer/981645030344.html',
    'https://detail.1688.com/offer/01.html', 'https://detail.1688.com/login.html'])('rejects unauthorized page %s', async url => {
    await expect(captureDocument(fixture().window.document, url, new Date())).rejects.toThrow();
  });

  it('rejects unknown/challenge pages instead of treating a page title as a product', async () => {
    const doc = new JSDOM('<title>请登录 / 验证码</title><h1>Product-looking title</h1>', { url: source }).window.document;
    await expect(captureDocument(doc, source, new Date())).rejects.toThrow('UNSUPPORTED_PAGE');
  });

  it('rejects source mismatch and oversized product fields', async () => {
    await expect(captureDocument(fixture().window.document, source.replace('981645030344', '9'), new Date())).rejects.toThrow();
    const dom = fixture();
    dom.window.document.querySelector('script')!.textContent = dom.window.document.querySelector('script')!.textContent!.replace('公开商品', 'x'.repeat(8193));
    await expect(captureDocument(dom.window.document, source, new Date())).rejects.toThrow();
  });
  it('rejects duplicate JSON keys, executable suffixes and over-budget static blocks', async () => {
    for (const mutate of [
      (raw:string)=>raw.replace('"title":', '"title":"shadow","title":'),
      (raw:string)=>`${raw};window.location="https://evil.test"`,
      ()=>`window.context = {"large":"${'x'.repeat(2*1024*1024)}"};`,
    ]) {
      const dom=fixture(); const node=dom.window.document.querySelector('script')!;
      node.textContent=mutate(node.textContent!);
      await expect(captureDocument(dom.window.document,source,new Date())).rejects.toThrow();
    }
  });

  it('rejects credential-shaped source attributes and candidate image query secrets', async () => {
    for (const [before,after] of [['材质','sessionToken'],['https://cbu01.alicdn.com/img/ibank/product.jpg','https://cbu01.alicdn.com/product.jpg?token=SECRET_CANARY']]) {
      const dom=fixture(); const node=dom.window.document.querySelector('script')!;
      node.textContent=node.textContent!.replace(before,after);
      await expect(captureDocument(dom.window.document,source,new Date())).rejects.toThrow();
    }
  });
});
