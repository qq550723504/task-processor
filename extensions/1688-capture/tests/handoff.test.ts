import { describe, expect, it } from 'vitest';
import { Handoff } from '../src/handoff';

const app = 'https://app.example.test/capture/1688';
const key = 'ab6203c2-e7f1-4daf-a758-a0cec2fb36a6';
const nonce = '0a7c4e7d-a8a1-4c5b-b6b5-b79df773a470';
const payload = { captureVersion: 1, evidence: { title: 'Fixture' } };
const sender = { origin: 'https://app.example.test', url: app, frameId: 0, tab: { id: 42 } };
const read = { version: 1, type: 'capture.read', handoffId: nonce, idempotencyKey: key };

describe('handoff is bounded transport, never publication authority', () => {
  it('fails closed before own tab binding and returns the same frozen payload/key after binding', () => {
    const flow = new Handoff(app, () => 1000);
    flow.prepare(payload, key, nonce);
    expect(() => flow.message(read, sender)).toThrow('INVALID_SENDER');
    flow.bindTab(42);
    expect(flow.message(read, sender)).toMatchObject({ type: 'capture.payload', idempotencyKey: key, payload });
    expect(flow.message(read, sender)).toEqual(flow.message(read, sender));
    expect(() => flow.prepare(payload, 'new-key', 'new-nonce')).toThrow('HANDOFF_ACTIVE');
  });

  it.each([
    { ...sender, origin: 'https://evil.test' }, { ...sender, url: `${app}/other` },
    { ...sender, url: 'https://app.example.test:8443/capture/1688' },
    { ...sender, frameId: 1 }, { ...sender, tab: { id: 43 } },
    { ...sender, origin: undefined }, { ...sender, id: 'another-extension' },
  ])('rejects mismatched or missing sender fields', bad => {
    const flow = new Handoff(app, () => 1000); flow.prepare(payload, key, nonce); flow.bindTab(42);
    expect(() => flow.message(read, bad)).toThrow('INVALID_SENDER');
  });

  it('binds status to the original handoff and key; unknown does not become a new operation', () => {
    const flow = new Handoff(app, () => 1000); flow.prepare(payload, key, nonce); flow.bindTab(42);
    expect(() => flow.message({ ...read, type: 'capture.status', idempotencyKey: 'other', outcome: 'published' }, sender)).toThrow();
    flow.message({ ...read, type: 'capture.status', outcome: 'outcome_unknown' }, sender);
    expect(flow.snapshot()).toMatchObject({ outcome: 'outcome_unknown', idempotencyKey: key });
    expect(flow.recoveryURL()).toBe(`${app}#operationKey=${key}`);
  });

  it('fails expired after TTL or worker restart, never inventing a recovery identity', () => {
    let now = 1000; const flow = new Handoff(app, () => now); flow.prepare(payload, key, nonce); flow.bindTab(42);
    now += 300001;
    expect(() => flow.message(read, sender)).toThrow('CAPTURE_EXPIRED');
    const restarted = new Handoff(app, () => now);
    expect(() => restarted.message(read, sender)).toThrow('CAPTURE_EXPIRED');
    expect(restarted.recoveryURL()).toBeNull();
  });

  it('does not regress unknown to processing or accept a different operation receipt', () => {
    const flow = new Handoff(app, () => 1000); flow.prepare(payload, key, nonce); flow.bindTab(42);
    flow.message({ ...read, type: 'capture.status', outcome: 'outcome_unknown', operationId: key }, sender);
    flow.message({ ...read, type: 'capture.status', outcome: 'processing', operationId: key }, sender);
    expect(flow.snapshot().outcome).toBe('outcome_unknown');
    expect(() => flow.message({ ...read, type: 'capture.status', outcome: 'published', operationId: nonce }, sender)).toThrow();
  });
});
