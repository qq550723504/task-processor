import {act,renderHook} from "@testing-library/react";
import {beforeEach,expect,it,vi} from "vitest";
import {usePODCommands} from "./commands";
import {MarketAPIError} from "@/lib/api/supply-market";
const {write,resolve,persist,clear,context}=vi.hoisted(()=>({write:vi.fn(),resolve:vi.fn(),persist:vi.fn(),clear:vi.fn(),context:{user:{id:"u"},effectiveOrganization:{id:"o"},registerOrganizationSwitchGuard:()=>()=>{}}}));
const intent={kind:"template" as const,key:"12752596-6056-4316-9f2f-380c97df9675",body:{id:"95146",hash:"a".repeat(64)}};
vi.mock("@/lib/api/pod",()=>({writePOD:write,resolvePOD:resolve}));
vi.mock("@/components/workbench/resources/resource-pending",()=>({useResourcePending:()=>({command:intent,ready:true,error:false,storageKey:"pod-test",persist,clear,read:()=>intent})}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>context}));
vi.mock("@tanstack/react-query",()=>({useQueryClient:()=>({invalidateQueries:vi.fn()})}));
beforeEach(()=>{vi.clearAllMocks();context.effectiveOrganization={id:"o"};Object.defineProperty(navigator,"locks",{configurable:true,value:{request:(_key:string,_options:unknown,run:(lock:object)=>unknown)=>run({})}})});
it("restored intent queries its original receipt and never writes again",async()=>{
 resolve.mockResolvedValue({operationId:intent.key,batchId:intent.key,itemId:intent.key,revision:1,replayed:true});
 const h=renderHook(()=>usePODCommands({userId:"u",organizationId:"o"}));await act(async()=>{await h.result.current.verify()});
 expect(resolve).toHaveBeenCalledWith({userId:"u",organizationId:"o"},intent,expect.any(AbortSignal));expect(write).not.toHaveBeenCalled();expect(clear).toHaveBeenCalledWith(intent);
});
it("missing receipt retains original intent and blocks a different submit",async()=>{
 resolve.mockRejectedValue(new MarketAPIError("NOT_FOUND",404));const h=renderHook(()=>usePODCommands({userId:"u",organizationId:"o"}));await act(async()=>{await h.result.current.verify();await h.result.current.execute({kind:"template",body:intent.body})});
 expect(clear).not.toHaveBeenCalled();expect(h.result.current.locked).toBe(true);expect(write).not.toHaveBeenCalled();
});
it("a context change during verification cannot clear or display the other scope's receipt",async()=>{
 let finish!:(v:unknown)=>void;resolve.mockReturnValue(new Promise(r=>{finish=r}));const h=renderHook(()=>usePODCommands({userId:"u",organizationId:"o"}));let request:Promise<unknown>|undefined;
 await act(async()=>{request=h.result.current.verify()});context.effectiveOrganization={id:"other"};h.rerender();await act(async()=>{finish({operationId:intent.key,revision:1,replayed:true});await request});
 expect(clear).not.toHaveBeenCalled();expect(h.result.current.saved).toBeNull();expect(write).not.toHaveBeenCalled();
});
