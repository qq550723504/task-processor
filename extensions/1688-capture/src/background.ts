import { Controller } from './controller';
import { pageSource, validateCapture } from './validation';
declare const APP_CAPTURE_URL: string;

async function extractCurrent() {
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tabs.length !== 1 || tabs[0].id === undefined || !tabs[0].url) throw Error('INVALID_PAGE');
  const tab = tabs[0]; const source = pageSource(tab.url!);
  const results = await chrome.scripting.executeScript({ target: { tabId: tab.id!, frameIds: [0] }, world: 'ISOLATED', files: ['extractor.js'] });
  const result = results[0]?.result as { ok?: boolean; payload?: unknown } | undefined;
  if (results.length !== 1 || results[0].frameId !== 0 || !results[0].documentId || !result?.ok) throw Error('CAPTURE_REJECTED');
  const payload = validateCapture(result.payload);
  // Verify the same document is still current without reading any other tab.
  const check = await chrome.scripting.executeScript({ target: { tabId: tab.id!, documentIds: [results[0].documentId] }, world: 'ISOLATED',
    func: () => document.URL });
  if (check.length !== 1 || check[0].documentId !== results[0].documentId || pageSource(check[0].result as string).sourceURL !== source.sourceURL
    || payload.evidence.sourceURL !== source.sourceURL) throw Error('PAGE_CHANGED');
  return payload;
}
const newController = () => new Controller(APP_CAPTURE_URL, extractCurrent, async url => {
  const tab = await chrome.tabs.create({ url });
  if (tab.id === undefined) throw Error('HANDOFF_UNAVAILABLE');
  return tab.id;
}, () => chrome.runtime.id);
let controller = newController();

chrome.runtime.onMessage.addListener((message, sender, respond) => {
  if (sender.id !== chrome.runtime.id || sender.url !== chrome.runtime.getURL('popup.html') || sender.tab
    || !message || Object.keys(message).length !== 1 || typeof message.type !== 'string') return false;
  (async () => {
    switch (message.type) {
      case 'popup.capture': await controller.capture(); break;
      case 'popup.handoff': await controller.handoff(); break;
      case 'popup.new': controller = newController(); break; // Explicit UI confirmation only.
      case 'popup.status': break;
      default: throw Error('INVALID_MESSAGE');
    }
    return controller.snapshot();
  })().then(state => respond({ ok: true, state }), () => respond({ ok: false, code: 'ACTION_UNAVAILABLE', state: controller.snapshot() }));
  return true; // Compatible asynchronous response in Chrome and Edge.
});
chrome.runtime.onMessageExternal.addListener((message, sender, respond) => {
  try { respond(controller.message(message, sender)); }
  catch { respond({ version: 1, code: 'CAPTURE_UNAVAILABLE' }); }
  return false;
});
