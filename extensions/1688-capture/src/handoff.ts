import { byteLength, fail, MAX_BYTES, UUID } from './validation';

type Sender = { origin?: string; url?: string; frameId?: number; tab?: { id?: number }; id?: string };
type Outcome = 'awaiting_confirmation' | 'processing' | 'published' | 'failed' | 'outcome_unknown';
type Pending = { payload: unknown; idempotencyKey: string; handoffId: string; expires: number; tabId?: number; outcome: Outcome; operationId?: string };
// An ephemeral transport only. It neither calls the API nor persists facts.
export class Handoff {
  private pending?: Pending;
  constructor(private readonly app: string, private readonly clock: () => number = Date.now) {}
  prepare(payload: unknown, key: string, nonce: string) {
    if (this.pending) fail('HANDOFF_ACTIVE');
    if (!UUID.test(key) || !UUID.test(nonce)) fail('INVALID_MESSAGE');
    const json = JSON.stringify(payload);
    if (byteLength(json) > MAX_BYTES) fail('CAPTURE_TOO_LARGE');
    this.pending = { payload: JSON.parse(json), idempotencyKey: key, handoffId: nonce, expires: this.clock() + 300000, outcome: 'awaiting_confirmation' };
  }
  bindTab(id: number) {
    if (!this.pending || this.pending.tabId !== undefined || !Number.isInteger(id) || id < 0) fail('INVALID_SENDER');
    this.pending.tabId = id;
  }
  private current() {
    if (!this.pending || this.clock() >= this.pending.expires) fail('CAPTURE_EXPIRED');
    return this.pending;
  }
  message(value: unknown, sender: Sender): unknown {
    const p = this.current();
    let url: URL;
    try { url = new URL(sender.url ?? ''); } catch { return fail('INVALID_SENDER'); }
    const app = new URL(this.app);
    if (sender.id !== undefined || p.tabId === undefined || sender.tab?.id !== p.tabId || sender.frameId !== 0
      || sender.origin !== app.origin || url.origin !== app.origin || url.pathname !== app.pathname || url.username || url.password || url.search) fail('INVALID_SENDER');
    if (!value || typeof value !== 'object' || Array.isArray(value)) fail('INVALID_MESSAGE');
    const m = value as Record<string, unknown>;
    if (m.version !== 1 || m.handoffId !== p.handoffId || m.idempotencyKey !== p.idempotencyKey) fail('INVALID_MESSAGE');
    const allowed = m.type === 'capture.read' ? ['version','type','handoffId','idempotencyKey'] : ['version','type','handoffId','idempotencyKey','outcome','operationId'];
    if (Object.keys(m).some(k => !allowed.includes(k))) fail('INVALID_MESSAGE');
    if (m.type === 'capture.read') return { version: 1, type: 'capture.payload', handoffId: p.handoffId, idempotencyKey: p.idempotencyKey, payload: structuredClone(p.payload) };
    if (m.type !== 'capture.status' || !['processing','published','failed','outcome_unknown'].includes(String(m.outcome))
      || (m.operationId !== undefined && (typeof m.operationId !== 'string' || !UUID.test(m.operationId)))) fail('INVALID_MESSAGE');
    // The page is a projection of the backend receipt, never a publication proof.
    if (m.outcome === 'published' && !m.operationId) fail('INVALID_MESSAGE');
    if (p.operationId && m.operationId !== undefined && p.operationId !== m.operationId) fail('INVALID_MESSAGE');
    if (p.outcome === 'published' || p.outcome === 'failed') return { version: 1, received: true };
    if (p.outcome === 'outcome_unknown' && m.outcome === 'processing') return { version: 1, received: true };
    p.outcome = m.outcome as Outcome;
    if (typeof m.operationId === 'string') p.operationId = m.operationId;
    return { version: 1, received: true };
  }
  snapshot() {
    const p = this.pending;
    if (!p) return { outcome: 'empty' as const };
    return { outcome: this.clock() >= p.expires ? 'outcome_unknown' : p.outcome, idempotencyKey: p.idempotencyKey, operationId: p.operationId };
  }
  handoffURL(extensionId: string) {
    const p = this.current(); const url = new URL(this.app);
    url.hash = new URLSearchParams({ extensionId, handoffId: p.handoffId, idempotencyKey: p.idempotencyKey }).toString();
    return url.href;
  }
  recoveryURL(): string | null {
    return this.pending ? `${this.app}#operationKey=${this.pending.idempotencyKey}` : null;
  }
}
