import { build } from 'esbuild';
import { mkdir, copyFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { appURL } from '../src/validation.ts';

const root = fileURLToPath(new URL('..', import.meta.url));
const fixture = process.argv.includes('--fixture');
const configured = process.env.CAPTURE_APP_URL;
if (!configured) throw Error('CAPTURE_APP_URL must explicitly name the application /capture/1688 URL');
const app = new URL(appURL(configured, fixture));
const outdir = resolve(root, fixture ? 'dist-fixture' : 'dist');
await mkdir(outdir, { recursive: true });
await build({ entryPoints: [resolve(root,'src/background.ts'),resolve(root,'src/popup.ts')], outdir, bundle:true, format:'esm', platform:'browser', target:'chrome120',
  define: { APP_CAPTURE_URL: JSON.stringify(app.href) } });
await build({ entryPoints:[resolve(root,'src/injected.ts')], outfile:resolve(outdir,'extractor.js'), bundle:true, format:'iife', globalName:'CaptureEntry', platform:'browser', target:'chrome120',
  footer: { js:'CaptureEntry.runCapture();' } });
const manifest = {
  manifest_version:3, name: fixture ? '1688 商品采集 — 任务 Fixture' : '1688 商品采集', version:'0.1.0', minimum_chrome_version:'120',
  description:'用户点击后采集当前1688商品资料，并在当前应用确认导入。', permissions:['activeTab','scripting'],
  action:{default_popup:'popup.html'}, background:{service_worker:'background.js',type:'module'},
  // Match patterns ignore ports: background additionally enforces exact origin.
  externally_connectable:{matches:[`${app.protocol}//${app.hostname}/capture/1688`]},
  content_security_policy:{extension_pages:"default-src 'none'; script-src 'self'; style-src 'self'; object-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'"},
};
await writeFile(resolve(outdir,'manifest.json'),JSON.stringify(manifest,null,2)+'\n');
for (const name of ['popup.html','popup.css']) await copyFile(resolve(root,'src',name),resolve(outdir,name));
console.log(`${fixture ? 'FIXTURE ONLY' : 'Application'} unpacked build: ${outdir}`);
