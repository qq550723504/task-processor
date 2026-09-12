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
  for(const key of ['host_permissions','optional_permissions','optional_host_permissions','content_scripts','web_accessible_resources']) expect(manifest[key]).toBeUndefined();
  expect(manifest.externally_connectable).toEqual({matches:['http://127.0.0.1/capture/1688']});
  expect(manifest.content_security_policy.extension_pages).toContain("connect-src 'none'");
  expect(manifest.content_security_policy.extension_pages).not.toMatch(/unsafe-inline|unsafe-eval/);
},20000);
