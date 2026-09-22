import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, it } from 'vitest';

it('builds only activeTab/scripting with exact app messaging, and rejects implicit or unsafe destinations', () => {
  const build=resolve('scripts/build.mjs');
  const env={...process.env};delete env.CAPTURE_APP_URL;
  expect(()=>execFileSync(process.execPath,[build],{env,stdio:'pipe'})).toThrow();
  for(const destination of ['http://app.example.com/capture/1688','https://user:password@app.example.com/capture/1688','https://app.example.com/other']) {
    expect(()=>execFileSync(process.execPath,[build],{env:{...env,CAPTURE_APP_URL:destination},stdio:'pipe'})).toThrow();
  }
  execFileSync(process.execPath,[build,'--fixture'],{env:{...env,CAPTURE_APP_URL:'http://127.0.0.1:4399/capture/1688'},stdio:'pipe'});
  const manifest=JSON.parse(readFileSync('dist-fixture/manifest.json','utf8'));
  expect(manifest.permissions).toEqual(['activeTab','scripting']);
  // The extension reads exactly one kind of page: detail.1688.com/offer/<id>.html, and the
  // extractor rejects every other URL itself. The host grant is therefore that one host,
  // https only, with no subdomain wildcard and no <all_urls>. Anything that could widen the
  // surface implicitly or silently stays absent below.
  expect(manifest.host_permissions).toEqual(['https://detail.1688.com/*']);
  for(const key of ['optional_permissions','optional_host_permissions','content_scripts','web_accessible_resources']) expect(manifest[key]).toBeUndefined();
  expect(manifest.externally_connectable).toEqual({matches:['http://127.0.0.1/capture/1688']});
  expect(manifest.content_security_policy.extension_pages).toContain("connect-src 'none'");
  expect(manifest.content_security_policy.extension_pages).not.toMatch(/unsafe-inline|unsafe-eval/);
  // The fixture build is not what ships. Assert the real build separately, so a permission
  // cannot be added to one and silently missed in the other.
  execFileSync(process.execPath,[build],{env:{...env,CAPTURE_APP_URL:'https://app.example.com/capture/1688'},stdio:'pipe'});
  const shipped=JSON.parse(readFileSync('dist/manifest.json','utf8'));
  expect(shipped.permissions).toEqual(manifest.permissions);
  expect(shipped.host_permissions).toEqual(manifest.host_permissions);
  expect(shipped.externally_connectable).toEqual({matches:['https://app.example.com/capture/1688']});
},20000);
