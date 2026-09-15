import { describe, expect, it } from 'vitest';
import { appURL, pageSource, publicImage, validateCapture } from '../src/validation';

export function minimalCapture() {
  return { captureVersion: 1, evidence: { schemaVersion: 1, sourceURL: 'https://detail.1688.com/offer/981645030344.html',
    offerID: '981645030344', title: 'Fixture', description: null, attributes: [], variants: [], priceFacts: [], images: [],
    capturedAt: '2026-09-12T00:00:00Z', contentSHA256: 'a'.repeat(64), parserVersion: '1688-browser-dom/v1',
    warnings: [{ code: 'MISSING_PRICE', field: 'priceFacts' }], missingFacts: [{ field: 'priceFacts', reason: 'not_observed' }] } };
}
describe('frozen wire and resource admission', () => {
  it('consumes lowercase wire; never accepts authority, secret or Go-domain aliases', () => {
    expect(validateCapture(minimalCapture()).evidence.title).toBe('Fixture');
    for (const field of ['org', 'actor', 'roles', 'catalogVersion', 'token', 'cookie']) {
      expect(() => validateCapture({ ...minimalCapture(), [field]: 'forged' })).toThrow();
      expect(() => validateCapture({ ...minimalCapture(), evidence: { ...minimalCapture().evidence, [field]: 'forged' } })).toThrow();
    }
    expect(() => validateCapture({ ...minimalCapture(), evidence: { ...minimalCapture().evidence, warnings: [{ Code: 'x', Field: 'y' }] } })).toThrow();
  });
  it('rejects null arrays, duplicate attributes and more than 256 collection items', () => {
    for (const attributes of [null, [{ name: 'x', value: '1' }, { name: 'x', value: '2' }], Array.from({ length: 257 }, (_, n) => ({ name: String(n), value: 'x' }))]) {
      expect(() => validateCapture({ ...minimalCapture(), evidence: { ...minimalCapture().evidence, attributes } })).toThrow();
    }
  });
  it.each(['https://localhost/x', 'https://127.0.0.1/x', 'https://[::1]/x', 'https://user:pw@img.example.com/x',
    'https://img.example.com/x?token=secret', 'javascript:alert(1)', 'http://img.example.com/x'])('rejects unsafe image %s', url => {
    expect(publicImage(url)).toBe(false);
  });
  it('canonicalizes only admitted current page URLs without query/fragment leakage', () => {
    expect(pageSource('http://DETAIL.1688.com/offer/99999999999999999999.html?token=secret#session')).toEqual({
      sourceURL: 'https://detail.1688.com/offer/99999999999999999999.html', offerID: '99999999999999999999',
    });
    for (const url of ['https://detail.1688.com/offer/%39.html', 'https://detail.1688.com:80/offer/9.html', 'https://detail.1688.com./offer/9.html', 'https://detail.1688.com/offer/9.html?bad=%xx']) expect(() => pageSource(url)).toThrow();
  });
  it('separates explicit HTTPS application builds from loopback fixture builds', () => {
    expect(appURL('https://app.example.com/capture/1688')).toBe('https://app.example.com/capture/1688');
    expect(() => appURL('http://127.0.0.1:4399/capture/1688')).toThrow();
    expect(appURL('http://127.0.0.1:4399/capture/1688', true)).toBe('http://127.0.0.1:4399/capture/1688');
    expect(() => appURL('https://app.example.com/capture/1688?target=https://evil.test')).toThrow();
  });
  it('rejects aggregate budget excess and multibyte strings over the byte ceiling', () => {
    const e = minimalCapture().evidence;
    expect(() => validateCapture({ captureVersion: 1, evidence: { ...e, title: '棉'.repeat(3000) } })).toThrow();
    const attributes=Array.from({length:256},(_,i)=>({name:`attr${i}`,value:'x'}));
    const variants=Array.from({length:4},(_,i)=>({sourceID:String(i),sku:null,title:null,attributes,price:null}));
    expect(() => validateCapture({captureVersion:1,evidence:{...e,attributes,variants}})).toThrow();
  });
});
