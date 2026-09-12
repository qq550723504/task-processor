import { describe, expect, it } from 'vitest';
import { JSDOM } from 'jsdom';
import { readFileSync } from 'node:fs';
import { captureDocument } from '../src/extractor';

const source = 'https://detail.1688.com/offer/981645030344.html';
function fixture(extra = '') {
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
      parserVersion: '1688-browser-dom/v1', title: '公开商品 <img onerror=alert(1)>',
      attributes: [{ name: '材质', value: '棉' }],
      variants: [{ sourceID: '99999999999999999999', price: { amount: '1.23000001', currency: null } }],
      priceFacts: [{ amount: '12.34000001', currency: null, minQuantity: '2' }],
      images: [{ url: 'https://cbu01.alicdn.com/img/ibank/product.jpg', role: 'source' }],
    });
    expect(payload.evidence.contentSHA256).toMatch(/^[a-f0-9]{64}$/);
    expect(payload.evidence.missingFacts.some(f => f.field.includes('currency'))).toBe(true);
  });

  it('keeps skuId as source identity without inventing a separately observed SKU', async () => {
    const payload=await captureDocument(fixture().window.document,source,new Date());
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
