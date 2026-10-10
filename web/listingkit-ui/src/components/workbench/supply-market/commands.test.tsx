import {act,renderHook} from "@testing-library/react";
import {beforeEach,expect,it,vi} from "vitest";
import {useMarketCommands} from "./shared";
import {MarketAPIError} from "@/lib/api/supply-market";
const {write,resolve,persist,clear}=vi.hoisted(()=>({write:vi.fn(),resolve:vi.fn(),persist:vi.fn(),clear:vi.fn()}));
vi.mock("@/lib/api/supply-market",async(importOriginal)=>({...await importOriginal<object>(),writeMarket:write,resolveMarket:resolve}));
const intent={key:"12752596-6056-4316-9f2f-380c97df9675",admin:false,command:{action:"select_release" as const,id:"12752596-6056-4316-9f2f-380c97df9675",expectedRevision:1}};
vi.mock("@/components/workbench/resources/resource-pending",()=>({useResourcePending:()=>({command:intent,ready:true,error:false,storageKey:"market-test",persist,clear,read:()=>intent})}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>({user:{id:"u"},effectiveOrganization:{id:"o"},registerOrganizationSwitchGuard:()=>()=>{}})}));
vi.mock("@tanstack/react-query",()=>({useQueryClient:()=>({invalidateQueries:vi.fn()})}));
beforeEach(()=>{vi.clearAllMocks()});
it("a restored unknown command queries its original receipt without sending another write",async()=>{
 resolve.mockResolvedValue({operationId:intent.key,revision:1,replayed:true});
 const h=renderHook(()=>useMarketCommands({userId:"u",organizationId:"o"}));
 await act(async()=>{await h.result.current.verify()});
 expect(resolve).toHaveBeenCalledWith({userId:"u",organizationId:"o"},intent,expect.any(AbortSignal));expect(write).not.toHaveBeenCalled();expect(clear).toHaveBeenCalledWith(intent);
});
it("a missing receipt keeps the unknown intent and blocks a different submit",async()=>{
 resolve.mockRejectedValue(new MarketAPIError("NOT_FOUND",404));
 const h=renderHook(()=>useMarketCommands({userId:"u",organizationId:"o"}));
 await act(async()=>{await h.result.current.verify()});
 expect(clear).not.toHaveBeenCalled();expect(h.result.current.locked).toBe(true);expect(write).not.toHaveBeenCalled();
});
