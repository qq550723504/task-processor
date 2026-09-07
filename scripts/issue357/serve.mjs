import {existsSync} from 'node:fs';
import {writeFile} from 'node:fs/promises';
import {join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import {readJSON,json,processIdentity,pause} from './io.mjs';
import {startChildren,terminateChildren} from './processes.mjs';

const dir=process.argv[2],cfg=await readJSON(join(dir,'services.json'));
const children=[],records={supervisor:await processIdentity(process.pid)};
await json(join(dir,'processes.json'),records);
let stopping=false;
async function stop(){
 if(stopping)return;stopping=true;
 await writeFile(join(dir,'stop-next'),'stop');
 const [go,next]=children;
 let end=Date.now()+15000;while(next?.pid&&next.exitCode===null&&next.signalCode===null&&Date.now()<end)await pause(100);
 await writeFile(join(dir,'stop-go'),'stop');end=Date.now()+15000;while(go.exitCode===null&&Date.now()<end)await pause(100);
 const passed=children.length===2&&children.every(p=>p.exitCode===0);
 await terminateChildren(children);
 await json(join(dir,'services-stopped.json'),{passed,goExit:go?.exitCode,nextExit:next?.exitCode});
 process.exitCode=passed?0:1;
}
try {
 await startChildren([
  {name:'go',command:cfg.binary,args:['-test.run=^TestIssue357Serve$','-test.timeout=24h'],env:{ISSUE357_INPUT_FILE:join(dir,'runtime.json')},cwd:dir},
  {name:'next',command:process.execPath,args:[join(dirname(fileURLToPath(import.meta.url)),'next.mjs'),dir],env:cfg.nextEnvironment,cwd:cfg.uiDirectory},
 ],async(name,identity)=>{records[name]=identity;await json(join(dir,'processes.json'),records)},children);
 while(!stopping){if(existsSync(join(dir,'stop-services'))||children.some(p=>p.exitCode!==null||p.signalCode!==null))await stop();await pause(200)}
}catch {
 await json(join(dir,'services-stopped.json'),{passed:false,reason:'service_start_or_stop_failed'}).catch(()=>{});
 process.exitCode=1;
}finally{await terminateChildren(children)}
