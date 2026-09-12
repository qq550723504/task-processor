export {};
type State = { loading: boolean; captured: boolean; handed: boolean; title: string | null; sourceURL: string | null;
  outcome: string; recoveryURL: string | null; missingFacts: { field: string; reason: string }[] };
const element = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;
const capture = element<HTMLButtonElement>('capture'); const handoff = element<HTMLButtonElement>('handoff');
const fresh = element<HTMLButtonElement>('fresh'); const status = element('status');
const recovery = element<HTMLAnchorElement>('recovery');
const outcomes: Record<string, string> = {
  empty: '打开 1688 商品详情页后采集。如之前已提交，请先在原应用页核实。', awaiting_confirmation: '已交给应用，请在应用中确认当前用户和企业。',
  processing: '应用正在导入；响应中断时请核实原操作。',
  published: '应用返回导入成功。可打开原操作查看结果。', failed: '应用返回导入失败，请查看原操作。',
  outcome_unknown: '结果待核实。请在应用中核实原操作，不要重复导入。',
};
function render(state: State) {
  element('title').textContent = state.title ?? '1688 商品采集';
  element('source').textContent = state.sourceURL ?? '';
  status.textContent = state.loading ? '正在读取当前商品…' : state.captured && !state.handed ? '采集完成。导入前请在应用中确认。' : outcomes[state.outcome] ?? '请在应用中核实结果。';
  capture.disabled = state.loading || state.captured;
  handoff.disabled = !state.captured || state.handed;
  fresh.hidden = !state.captured;
  recovery.hidden = !state.recoveryURL;
  if (state.recoveryURL) recovery.href = state.recoveryURL;
  const labels: Record<string,string> = { description:'商品描述',attributes:'商品属性',variants:'规格',priceFacts:'报价',images:'来源图片' };
  const missing = [...new Set(state.missingFacts.map(item => item.field.includes('currency') ? '报价币种' : labels[item.field] ?? '部分商品资料'))].join('、');
  element('missing').textContent = missing ? `未取得：${missing}` : '';
}
async function action(type: string) {
  capture.disabled = true; handoff.disabled = true; status.textContent = '正在处理…';
  try {
    const response = await chrome.runtime.sendMessage({ type });
    if (response?.state) render(response.state);
    if (!response?.ok) status.textContent = '无法完成本次操作。可能是页面不支持、结构变化或采集已过期；如已交给应用，请核实原操作。';
  } catch {
    status.textContent = '采集状态已失效。如已提交，请在原应用页核实；未提交时可显式重新采集。'; capture.disabled = false;
  }
}
capture.addEventListener('click', () => void action('popup.capture'));
handoff.addEventListener('click', () => void action('popup.handoff'));
fresh.addEventListener('click', () => { element('new-confirm').hidden = false; });
element('new-cancel').addEventListener('click', () => { element('new-confirm').hidden = true; });
element('new-confirm-button').addEventListener('click', () => { element('new-confirm').hidden = true; void action('popup.new'); });
element('refresh').addEventListener('click', () => void action('popup.status'));
void action('popup.status');
