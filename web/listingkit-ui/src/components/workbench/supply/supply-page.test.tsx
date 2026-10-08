import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SupplyPage } from "./supply-page";
const state=vi.hoisted(()=>({context:{} as Record<string,unknown>,preparation:vi.fn(),sources:vi.fn(),history:vi.fn(),stores:vi.fn(),options:vi.fn(),write:vi.fn()}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>state.context}));
vi.mock("@/lib/api/supply-chain",async original=>({...await original<object>(),readSupplyPreparation:state.preparation,listSupplySources:state.sources,listSupplyOperations:state.history,supplyCommand:state.write,readSupplyOptimizationOptions:state.options}));
vi.mock("@/lib/api/workbench-stores",()=>({listWorkbenchStores:state.stores}));
const prep="11111111-1111-4111-8111-111111111111",store="22222222-2222-4222-8222-222222222222",source="33333333-3333-4333-8333-333333333333";
beforeEach(()=>{
 state.context={user:{id:"actor-a"},effectiveOrganization:{id:"org-a"},permissions:["workbench.supply.read","workbench.supply.manage","workbench.listing.submit","workbench.store.read"],supplyIntentReady:true,pendingSupplyIntent:null,pendingCollectionIntent:null,setPendingSupplyIntent:()=>true,registerOrganizationSwitchGuard:()=>()=>{}};
 state.options.mockReset().mockResolvedValue({titles:[],reason:"未启用"});
 state.preparation.mockReset().mockResolvedValue({id:prep,sourceBatchId:prep,sourceRevision:1,name:"完整批次",count:205,revision:1,createdAt:"2026-10-08T00:00:00Z"});
 state.sources.mockReset().mockResolvedValue({items:[],total:205,nextCursor:source});state.history.mockReset().mockResolvedValue({items:[],total:0});state.write.mockReset().mockResolvedValue({operation:{id:source,input:{preparationId:prep,expectedRevision:1,storeId:store,action:"adapt"},count:205,completed:0,status:"pending",execution:"started",createdAt:"2026-10-08T00:00:00Z"},replayed:false});
 state.stores.mockReset().mockResolvedValue({items:[{id:store,name:"美国试用店铺",platform:"shein",recordStatus:"active",connectionStatus:"connected",serviceStatus:"active"}],pagination:{page:1,pageSize:100,total:1}});
});afterEach(cleanup);
it("reads a retained batch and starts a whole-batch action without truncating it to the displayed page",async()=>{
 render(<SupplyPage initialPreparation={prep}/>);await screen.findByText("完整批次");
 await userEvent.selectOptions(await screen.findByLabelText("目标店铺"),store);
 await userEvent.click(screen.getByRole("button",{name:"适配整批"}));
 await userEvent.click(screen.getByRole("button",{name:"确认适配"}));
 await waitFor(()=>expect(state.write).toHaveBeenCalledOnce());
 expect(state.write.mock.calls[0][0].command).toEqual({preparationId:prep,expectedRevision:1,storeId:store,action:"adapt"});
 expect(state.preparation).toHaveBeenCalledWith({userId:"actor-a",organizationId:"org-a"},prep,expect.any(AbortSignal));
});
it("keeps supply writes unavailable without the management grant",async()=>{
 state.context.permissions=["workbench.supply.read","workbench.store.read"];
 render(<SupplyPage initialPreparation={prep}/>);await screen.findByText("完整批次");
 await userEvent.selectOptions(await screen.findByLabelText("目标店铺"),store);
 expect(screen.getByRole("button",{name:"适配整批"})).toBeDisabled();
});

it("confirms full-batch retail points and pins the exact selected title template quote",async()=>{
 state.options.mockResolvedValue({titles:[{agentId:"product.title.agent",templateId:source,revision:"3",name:"美国站标题",quoteHash:"a".repeat(64),maximumPoints:10,priceVersion:"price-1",maximumCostMicros:1000,currency:"USD"}]});
 render(<SupplyPage initialPreparation={prep}/>);await screen.findByText("完整批次");
 await userEvent.selectOptions(await screen.findByLabelText("目标店铺"),store);
 await userEvent.click(screen.getByLabelText("开启智能体优化"));
 await userEvent.selectOptions(await screen.findByLabelText("标题优化模板"),`${source}:3`);
 await userEvent.click(screen.getByRole("button",{name:"重构整批"}));
 expect(screen.getByText(/本次预算上限 2050 积分/)).toBeInTheDocument();
 await userEvent.click(screen.getByRole("button",{name:"确认智能体优化"}));
 await waitFor(()=>expect(state.write).toHaveBeenCalledOnce());
 expect(state.write.mock.calls[0][0].command).toEqual({preparationId:prep,expectedRevision:1,storeId:store,action:"optimize",titleTemplateId:source,titleTemplateRevision:"3",titleQuoteHash:"a".repeat(64)});
});
