import {act,cleanup,renderHook,waitFor} from "@testing-library/react";
import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import type {ReactNode} from "react";
import {ecoRequest,EcoservicesError} from "@/lib/api/ecoservices";
import {useEcoCommands} from "./shared";

const context=vi.hoisted(()=>({user:{id:"actor"},effectiveOrganization:{id:"org"},isLoading:false,isSwitching:false,error:null,blockingError:null,registerOrganizationSwitchGuard:vi.fn(()=>()=>undefined)}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>context}));
vi.mock("@/lib/api/ecoservices",async original=>({...await original<typeof import("@/lib/api/ecoservices")>(),ecoRequest:vi.fn()}));
const scope={userId:"actor",organizationId:"org"},id="4841d296-ef14-4c16-8d25-a7667e534feb";
function harness(){const client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});return {client,wrapper:({children}:{children:ReactNode})=><QueryClientProvider client={client}>{children}</QueryClientProvider>}}
beforeEach(()=>{
 localStorage.clear();sessionStorage.clear();vi.resetAllMocks();context.user={id:"actor"};context.effectiveOrganization={id:"org"};
 const active=new Set<string>();
 Object.defineProperty(navigator,"locks",{configurable:true,value:{request:async(name:string,_options:unknown,run:(lock:unknown)=>Promise<unknown>)=>{if(active.has(name))return run(null);active.add(name);try{return await run({name})}finally{active.delete(name)}}}});
});
afterEach(()=>{cleanup();vi.restoreAllMocks();localStorage.clear();sessionStorage.clear()});
it("restores an ambiguous create in a fresh client, without automatic dispatch or a changed key",async()=>{
 vi.mocked(ecoRequest).mockRejectedValueOnce(new EcoservicesError("OUTCOME_UNKNOWN"));
 const first=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:first.wrapper});
 await act(async()=>{await a.result.current.json("catalog/"+id+"/requests",{description:"原需求",fileIds:[]}).catch(()=>undefined)});
 const original=vi.mocked(ecoRequest).mock.calls[0];a.unmount();first.client.clear();
 const second=harness(),b=renderHook(()=>useEcoCommands(scope),{wrapper:second.wrapper});
 await waitFor(()=>expect(b.result.current.intent).toBeTruthy());expect(ecoRequest).toHaveBeenCalledTimes(1);
 await expect(b.result.current.json("catalog/"+id+"/requests",{description:"不同需求",fileIds:[]})).rejects.toBeTruthy();expect(ecoRequest).toHaveBeenCalledTimes(1);
 vi.mocked(ecoRequest).mockResolvedValueOnce({});await act(async()=>{await b.result.current.execute(b.result.current.intent!)});
 const retry=vi.mocked(ecoRequest).mock.calls[1];expect(retry[1]).toBe(original[1]);expect(retry[3]?.body).toBe(original[3]?.body);expect(new Headers(retry[3]?.headers).get("Idempotency-Key")).toBe(new Headers(original[3]?.headers).get("Idempotency-Key"));
 await waitFor(()=>expect(b.result.current.intent).toBeNull());expect(localStorage.length).toBe(0);second.client.clear();
});
it.each([new EcoservicesError("ECOSERVICES_UNAVAILABLE",503),new EcoservicesError("INVALID_UPSTREAM_RESPONSE",502),new EcoservicesError("ECOSERVICES_FORBIDDEN",403),new EcoservicesError("IDENTITY_CONTEXT_CHANGED",409),new EcoservicesError("ORGANIZATION_CONTEXT_CHANGED",409)])("retains the original body and CAS after an uncertain or revoked response %s",async(error)=>{
 vi.mocked(ecoRequest).mockRejectedValueOnce(error);const first=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:first.wrapper});
 await act(async()=>{await a.result.current.json("requests/"+id+"/accept",{deliveryVersion:"3"},"7").catch(()=>undefined)});
 a.unmount();first.client.clear();const second=harness(),b=renderHook(()=>useEcoCommands(scope),{wrapper:second.wrapper});
 await waitFor(()=>expect(b.result.current.intent).toMatchObject({body:'{"deliveryVersion":"3"}',version:"7"}));expect(ecoRequest).toHaveBeenCalledTimes(1);second.client.clear();
});
it("shares the pending intent across hooks and excludes concurrent dispatch",async()=>{
 let release!:()=>void;vi.mocked(ecoRequest).mockImplementationOnce(()=>new Promise(resolve=>{release=()=>resolve({})}));
 const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper}),b=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 let pending!:Promise<unknown>;await act(async()=>{pending=a.result.current.json("catalog/"+id+"/requests",{description:"原需求",fileIds:[]})});
 await waitFor(()=>expect(b.result.current.locked).toBe(true));
 await expect(b.result.current.json("catalog/"+id+"/requests",{description:"不同需求",fileIds:[]})).rejects.toBeTruthy();
 expect(ecoRequest).toHaveBeenCalledTimes(1);await act(async()=>{release();await pending});h.client.clear();
});
it("refuses dispatch if the original intent cannot be saved",async()=>{
 vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw new Error("quota unavailable")});
 const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 await act(async()=>{await a.result.current.json("catalog/"+id+"/requests",{description:"原需求",fileIds:[]}).catch(()=>undefined)});expect(ecoRequest).not.toHaveBeenCalled();h.client.clear();
});
it.each([["malformed","corrupt"],["oversized","x".repeat(131073)]])("keeps %s saved commands fenced instead of deleting them or generating a new operation",async(_name,value)=>{
 vi.mocked(ecoRequest).mockRejectedValueOnce(new EcoservicesError("OUTCOME_UNKNOWN"));const first=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:first.wrapper});
 await act(async()=>{await a.result.current.json("requests/"+id+"/accept",{deliveryVersion:"3"},"7").catch(()=>undefined)});
 expect(localStorage.length).toBe(1);const key=localStorage.key(0)!;localStorage.setItem(key,value);a.unmount();first.client.clear();
 const second=harness(),b=renderHook(()=>useEcoCommands(scope),{wrapper:second.wrapper});await waitFor(()=>expect(b.result.current.locked).toBe(true));
 await expect(b.result.current.json("requests/"+id+"/accept",{deliveryVersion:"4"},"8")).rejects.toBeTruthy();expect(ecoRequest).toHaveBeenCalledTimes(1);expect(localStorage.getItem(key)).toBe(value);second.client.clear();
});
it("does not expose another user or organization's pending command and rejects a stale live context",async()=>{
 vi.mocked(ecoRequest).mockRejectedValueOnce(new EcoservicesError("OUTCOME_UNKNOWN"));const first=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:first.wrapper});
 await act(async()=>{await a.result.current.json("requests/"+id+"/accept",{deliveryVersion:"3"},"7").catch(()=>undefined)});const intent=a.result.current.intent!;
 context.user={id:"other"};context.effectiveOrganization={id:"other-org"};const second=harness(),b=renderHook(()=>useEcoCommands({userId:"other",organizationId:"other-org"}),{wrapper:second.wrapper});
 expect(b.result.current.intent).toBeNull();await expect(a.result.current.execute(intent)).rejects.toBeTruthy();expect(ecoRequest).toHaveBeenCalledTimes(1);first.client.clear();second.client.clear();
});
it("does not write merchant identity/bank data or file bytes into durable browser storage",async()=>{
 vi.mocked(ecoRequest).mockRejectedValue(new EcoservicesError("OUTCOME_UNKNOWN"));const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 await act(async()=>{await a.result.current.execute({path:"applications/"+id+"/merchant",key:crypto.randomUUID(),body:'{"accountNumber":"private-bank","legal":{"number":"private-id"}}',output:"merchant"}).catch(()=>undefined)});
 expect(localStorage.length).toBe(0);a.unmount();h.client.clear();const second=harness(),b=renderHook(()=>useEcoCommands(scope),{wrapper:second.wrapper});
 const body=new FormData();body.set("file",new File(["private-file"],"license.png"));await act(async()=>{await b.result.current.execute({path:"applications/files",key:crypto.randomUUID(),body,output:"file"}).catch(()=>undefined)});expect(localStorage.length).toBe(0);second.client.clear();
});
it("keeps the current maximum JSON input usable and clears only a definite original rejection",async()=>{
 vi.mocked(ecoRequest).mockRejectedValueOnce(new EcoservicesError("ECOSERVICES_CONFLICT",409));const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 await act(async()=>{await a.result.current.json("catalog/"+id+"/requests",{description:"需".repeat(10000),fileIds:[]}).catch(()=>undefined)});
 expect(ecoRequest).toHaveBeenCalledTimes(1);expect(localStorage.length).toBe(0);h.client.clear();
});
it("preserves the largest escaped valid quote through outer pending JSON encoding",async()=>{
 vi.mocked(ecoRequest).mockRejectedValueOnce(new EcoservicesError("OUTCOME_UNKNOWN"));const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 await act(async()=>{await a.result.current.json("provider/requests/"+id+"/quote",{amountMinor:"100",scope:"\u0001".repeat(9999)+"x",acceptanceCriteria:"\u0001".repeat(4999)+"x",deliveryDays:1},"7").catch(()=>undefined)});
 expect(ecoRequest).toHaveBeenCalledTimes(1);expect(a.result.current.intent).toBeTruthy();h.client.clear();
});
it("refuses a dispatch without browser locking support",async()=>{
 Object.defineProperty(navigator,"locks",{configurable:true,value:undefined});const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 await act(async()=>{await a.result.current.json("catalog/"+id+"/requests",{description:"原需求",fileIds:[]}).catch(()=>undefined)});expect(ecoRequest).not.toHaveBeenCalled();h.client.clear();
});
it("rechecks the live identity after acquiring the lock, before saving or dispatching",async()=>{
 let release!:(value:unknown)=>void;
 Object.defineProperty(navigator,"locks",{configurable:true,value:{request:async(_name:string,_options:unknown,run:(lock:unknown)=>Promise<unknown>)=>{await new Promise(resolve=>{release=resolve});return run({})}}});
 const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});let pending!:Promise<unknown>;
 await act(async()=>{pending=a.result.current.json("catalog/"+id+"/requests",{description:"原需求",fileIds:[]}).catch(error=>error)});
 context.user={id:"other"};a.rerender();await act(async()=>{release(null);await pending});expect(ecoRequest).not.toHaveBeenCalled();expect(localStorage.length).toBe(0);h.client.clear();
});
it("retains a known original result when storage cleanup fails, and only retries its same key",async()=>{
 vi.mocked(ecoRequest).mockResolvedValue({});const h=harness(),a=renderHook(()=>useEcoCommands(scope),{wrapper:h.wrapper});
 const clear=vi.spyOn(Storage.prototype,"removeItem").mockImplementation(()=>{throw new Error("storage unavailable")});
 await act(async()=>{await a.result.current.json("catalog/"+id+"/requests",{description:"原需求",fileIds:[]}).catch(()=>undefined)});
 expect(a.result.current.intent).toBeTruthy();expect(localStorage.length).toBe(1);const first=vi.mocked(ecoRequest).mock.calls[0];clear.mockRestore();
 await act(async()=>{await a.result.current.execute(a.result.current.intent!)});expect(new Headers(vi.mocked(ecoRequest).mock.calls[1][3]?.headers).get("Idempotency-Key")).toBe(new Headers(first[3]?.headers).get("Idempotency-Key"));expect(localStorage.length).toBe(0);h.client.clear();
});
