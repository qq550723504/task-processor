import { expect, it, vi } from 'vitest';
import { Controller } from '../src/controller';
import type { CapturePayload } from '../src/wire';

const payload: CapturePayload = { captureVersion: 1, evidence: { schemaVersion: 1,
  sourceURL: 'https://detail.1688.com/offer/9.html', offerID: '9', title: 'Observed', description: null,
  attributes: [], variants: [], priceFacts: [], images: [], capturedAt: '2026-09-12T00:00:00Z',
  contentSHA256: 'a'.repeat(64), parserVersion: '1688-browser-dom/v1', warnings: [], missingFacts: [] } };

it('coalesces repeated capture clicks and never injects on a rejected current page', async () => {
  let resolve!: (v: CapturePayload) => void;
  const capture = vi.fn(() => new Promise<CapturePayload>(r => { resolve = r; }));
  const createTab = vi.fn(async () => 42);
  const control = new Controller('https://app.example.com/capture/1688', capture, createTab, () => 'extension-id');
  const first = control.capture(); const second = control.capture();
  expect(capture).toHaveBeenCalledTimes(1);
  resolve(payload); await Promise.all([first, second]);
  await control.capture(); expect(capture).toHaveBeenCalledTimes(1);
  const summary = control.snapshot(); expect(summary.title).toBe('Observed');
  await Promise.all([control.handoff(), control.handoff()]); expect(createTab).toHaveBeenCalledTimes(1);
  const recovery = control.recoveryURL();
  await control.handoff(); expect(control.recoveryURL()).toBe(recovery); expect(createTab).toHaveBeenCalledTimes(1);
});

it('retains an original recovery key if creating the application tab fails', async () => {
  const control = new Controller('https://app.example.com/capture/1688', async () => payload, async () => { throw Error('network irrelevant'); }, () => 'extension-id');
  await control.capture(); await expect(control.handoff()).rejects.toThrow();
  expect(control.recoveryURL()).toMatch(/#operationKey=/);
  expect(control.snapshot().outcome).toBe('outcome_unknown');
});
