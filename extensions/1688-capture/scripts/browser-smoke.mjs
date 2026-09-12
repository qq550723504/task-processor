// Task-only browser harness. It substitutes the public page response and a
// transport-only receiver; it cannot prove Web/BFF/SRC1/Catalog acceptance.
import { chromium } from '@playwright/test';
import CDP from 'chrome-remote-interface';
import { createServer } from 'node:http';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';

const root = fileURLToPath(new URL('..', import.meta.url));
const head = execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8'}).trim();
const browserName = process.argv.includes('--edge') ? 'edge' : 'chrome';
const automated = process.argv.includes('--cdp');
const executablePath = browserName === 'edge' ? 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe' : 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const artifacts = resolve(root, 'artifacts', `${browserName}-${Date.now()}`);
await mkdir(artifacts, { recursive: true });
const receiver = `<!doctype html><html lang="zh-CN"><head><meta charset="UTF-8"><meta name="referrer" content="no-referrer"><title>Issue399 Fixture Receiver</title></head><body>
<h1>Transport Fixture — 不是真实后端</h1><p id="status">等待受限capture消息</p><button id="unknown" disabled>模拟响应丢失</button><pre id="summary"></pre>
<script>
const params = new URLSearchParams(location.hash.slice(1));
const extensionId=params.get('extensionId'),handoffId=params.get('handoffId'),idempotencyKey=params.get('idempotencyKey');
window.fixtureReceived=null;
async function readCapture(){
  if(!extensionId){document.querySelector('#status').textContent='恢复模式：只有原key，无自动POST';return;}
  let response;
  for(let attempt=0;attempt<3;attempt++){
    response=await chrome.runtime.sendMessage(extensionId,{version:1,type:'capture.read',handoffId,idempotencyKey});
    if(response?.type==='capture.payload')break;
    await new Promise(resolve=>setTimeout(resolve,250));
  }
  if(response?.type!=='capture.payload'){document.querySelector('#status').textContent='CAPTURE_UNAVAILABLE';return;}
  window.fixtureReceived=response;
  history.replaceState(null,'','#operationKey='+idempotencyKey);
  document.querySelector('#summary').textContent=JSON.stringify(response.payload,null,2);
  document.querySelector('#status').textContent='FIXTURE_CAPTURE_RECEIVED';document.querySelector('#unknown').disabled=false;
}
document.querySelector('#unknown').onclick=async()=>{
  await chrome.runtime.sendMessage(extensionId,{version:1,type:'capture.status',handoffId,idempotencyKey,outcome:'outcome_unknown'});
  document.querySelector('#status').textContent='FIXTURE_OUTCOME_UNKNOWN';
};
readCapture().catch(()=>document.querySelector('#status').textContent='CAPTURE_UNAVAILABLE');
</script></body></html>`;
const server = createServer((req, res) => {
  if (req.method !== 'GET' || req.url !== '/capture/1688') { res.writeHead(404); res.end(); return; }
  res.writeHead(200, { 'Content-Type':'text/html; charset=utf-8', 'Referrer-Policy':'no-referrer' }); res.end(receiver);
});
await new Promise((done, reject) => { server.once('error',reject); server.listen(4399,'127.0.0.1',done); });
let context;
try {
  context = await chromium.launchPersistentContext(resolve(artifacts, 'profile'), {
    executablePath, headless:false, ignoreDefaultArgs:['--disable-extensions'], viewport:null,
    args:['--no-first-run','--no-default-browser-check','--disable-background-networking', ...(automated ? ['--enable-unsafe-extension-debugging','--remote-debugging-port=4398'] : [])],
  });
  await context.route('**/*', async route => {
    const url = new URL(route.request().url());
    if (url.origin === 'https://detail.1688.com' && url.pathname === '/offer/981645030344.html') {
      await route.fulfill({ contentType:'text/html; charset=utf-8', body:await readFile(resolve(root,'tests/fixtures/product.html'),'utf8') });
    } else if (url.origin === 'http://127.0.0.1:4399' || url.protocol === 'chrome-extension:') await route.continue();
    else await route.abort();
  });
  const page = context.pages()[0];
  if (!automated) await page.goto(browserName === 'edge' ? 'edge://extensions' : 'chrome://extensions');
  console.log(JSON.stringify({ stage:'LOAD_UNPACKED_IN_TASK_PROFILE', browserName, extensionPath:resolve(root,'dist-fixture'), artifacts }));
  let browserCDP;
  let appPromise;
  if (automated) {
    browserCDP = await context.browser().newBrowserCDPSession();
    console.log(await browserCDP.send('Extensions.loadUnpacked',{path:resolve(root,'dist-fixture')}));
  }
  const worker = context.serviceWorkers()[0] ?? await context.waitForEvent('serviceworker',{timeout:600000});
  const id = worker.url().split('/')[2];
  const manifest = await worker.evaluate(() => chrome.runtime.getManifest());
  if (JSON.stringify(manifest.permissions) !== JSON.stringify(['activeTab','scripting']) || manifest.host_permissions) throw Error('Unexpected permissions');
  await page.goto('https://detail.1688.com/offer/981645030344.html');
  console.log(JSON.stringify({ stage:'CLICK_EXTENSION_ACTION_AND_CAPTURE', id }));
  if (automated) {
    const {targetInfos:tabs}=await browserCDP.send('Target.getTargets',{filter:[{type:'tab',exclude:false}]});
    if(tabs.length!==1)throw Error('Expected exactly one task source tab');
    await browserCDP.send('Extensions.triggerAction',{id,targetId:tabs[0].targetId});
    let target;
    for(let attempt=0;attempt<30;attempt++){
      target=(await CDP.List({port:4398})).find(t=>t.url===`chrome-extension://${id}/popup.html`);
      if(target)break;
      await new Promise(resolve=>setTimeout(resolve,100));
    }
    if(!target)throw Error('Extension action popup did not open');
    const popup=await CDP({port:4398,target:target.id});
    const evaluate=async expression=>{
      const result=await popup.Runtime.evaluate({expression,returnByValue:true,awaitPromise:true,userGesture:true});
      if(result.exceptionDetails)throw Error(JSON.stringify(result.exceptionDetails));return result.result.value;
    };
    console.log(JSON.stringify({stage:'POPUP_OPEN',text:await evaluate('document.body.innerText')}));
    await evaluate("document.querySelector('#capture').click()");
    for(let attempt=0;attempt<50;attempt++){
      if(await evaluate("!document.querySelector('#handoff').disabled"))break;
      await new Promise(resolve=>setTimeout(resolve,100));
    }
    console.log(JSON.stringify({stage:'CAPTURE_UI',text:await evaluate('document.body.innerText')}));
    if(await evaluate("document.querySelector('#handoff').disabled"))throw Error('Capture did not enable handoff');
    const shot=await popup.Page.captureScreenshot();await writeFile(resolve(artifacts,'popup.png'),Buffer.from(shot.data,'base64'));
    appPromise=context.waitForEvent('page',{timeout:20000});
    await evaluate("document.querySelector('#handoff').click()").catch(()=>{});
    await popup.close();
  }
  const app = await (appPromise ?? context.waitForEvent('page', { timeout:600000 })).catch(async () => {
    const found = context.pages().find(p=>p.url().startsWith('http://127.0.0.1:4399'));
    if(!found)throw Error('No own receiver tab');return found;
  });
  await app.waitForURL('http://127.0.0.1:4399/capture/1688**');
  await app.locator('#status').filter({hasText:'FIXTURE_CAPTURE_RECEIVED'}).waitFor({timeout:20000});
  const capture = await app.evaluate(()=>window.fixtureReceived);
  if (!capture || /PASSWORD_CANARY|AUTH_CANARY|TOKEN_CANARY|SESSION_CANARY/.test(JSON.stringify(capture))) throw Error('Missing or sensitive capture');
  if (capture.payload.evidence.variants[0].sourceID !== '99999999999999999999') throw Error('SKU precision lost');
  const wrong = await context.newPage(); await wrong.goto(app.url());
  const rejection = await wrong.evaluate(async ({id,capture})=>chrome.runtime.sendMessage(id,{version:1,type:'capture.read',handoffId:capture.handoffId,idempotencyKey:capture.idempotencyKey}),{id,capture});
  if(rejection.code!=='CAPTURE_UNAVAILABLE')throw Error('Other application tab read accepted');
  await wrong.close();
  await app.locator('#unknown').click();
  await app.locator('#status').filter({hasText:'FIXTURE_OUTCOME_UNKNOWN'}).waitFor();
  await app.screenshot({path:resolve(artifacts,'receiver.png')});
  const recoveryURL=app.url();
  if(!recoveryURL.endsWith(`#operationKey=${capture.idempotencyKey}`))throw Error('Original recovery key was lost');
  let workerRestartRejected=false;
  if(automated){
    const control=await context.newCDPSession(page);
    await control.send('ServiceWorker.enable');
    await control.send('ServiceWorker.stopAllWorkers');
    const afterStop=await app.evaluate(async ({id,capture})=>chrome.runtime.sendMessage(id,{version:1,type:'capture.read',handoffId:capture.handoffId,idempotencyKey:capture.idempotencyKey}),{id,capture});
    if(afterStop.code!=='CAPTURE_UNAVAILABLE')throw Error('Worker restart retained stale handoff');
    workerRestartRejected=true;
  }
  await app.reload();
  await app.locator('#status').filter({hasText:'恢复模式：只有原key，无自动POST'}).waitFor();
  if(app.url()!==recoveryURL)throw Error('Reload changed recovery identity');
  await writeFile(resolve(artifacts,'capture.json'),JSON.stringify(capture.payload,null,2));
  const buildHashes={};
  for(const name of ['manifest.json','background.js','extractor.js','popup.js','popup.html','popup.css']) buildHashes[name]=createHash('sha256').update(await readFile(resolve(root,'dist-fixture',name))).digest('hex');
  await writeFile(resolve(artifacts,'receipt.json'),JSON.stringify({head,buildHashes,browserName,browserVersion:context.browser()?.version(),id,manifest,
    result:'PASS_TRANSPORT_FIXTURE_ONLY',ownTabCapture:true,wrongAppTabRejected:true,unknownStatusSent:true,workerRestartRejected,reloadPreservedKey:true,
    backendIntegration:'NOT_RUN',real1688:'NOT_RUN'},null,2));
  console.log(JSON.stringify({stage:'TRANSPORT_FIXTURE_PASS',artifacts}));
} finally {
  if(context)await context.close();
  await new Promise(done=>server.close(done));
}
