import { captureDocument } from './extractor';
export async function runCapture() {
  try { return { ok: true, payload: await captureDocument(document, document.URL, new Date()) }; }
  catch { return { ok: false, code: 'CAPTURE_REJECTED' }; }
}
