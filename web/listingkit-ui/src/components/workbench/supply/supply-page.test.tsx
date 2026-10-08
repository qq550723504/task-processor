import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SupplyPage } from "./supply-page";
const state=vi.hoisted(()=>({context:{} as Record<string,unknown>,preparation:vi.fn(),stages:vi.fn(),sources:vi.fn(),history:vi.fn(),stores:vi.fn(),options:vi.fn(),write:vi.fn(),push:vi.fn(),detail:vi.fn(),target:vi.fn(),publication:vi.fn(),operation:vi.fn(),items:vi.fn(),attempt:vi.fn()}));
vi.mock("next/navigation",()=>({useRouter:()=>({push:state.push})}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>state.context}));
vi.mock("@/lib/api/supply-chain",async original=>({...await original<object>(),readSupplyPreparation:state.preparation,listSupplyStages:state.stages,listSupplySources:state.sources,listSupplyOperations:state.history,supplyCommand:state.write,readSupplyOptimizationOptions:state.options,readSupplySource:state.detail,readSupplyTarget:state.target,readSupplyPublication:state.publication,readSupplyOperation:state.operation,listSupplyOperationItems:state.items,readSupplyUploadAttempt:state.attempt}));
vi.mock("./bulk-review-panel",()=>({BulkSupplyReviewPanel:({items}:{items:unknown[]})=><p>实际批量提案 {items.length} 件</p>}));
vi.mock("@/lib/api/workbench-stores",()=>({listWorkbenchStores:state.stores}));
const prep="11111111-1111-4111-8111-111111111111",store="22222222-2222-4222-8222-222222222222",source="33333333-3333-4333-8333-333333333333";
beforeEach(()=>{
 state.context={user:{id:"actor-a"},effectiveOrganization:{id:"org-a"},permissions:["workbench.supply.read","workbench.supply.manage","workbench.listing.submit","workbench.store.read"],supplyIntentReady:true,pendingSupplyIntent:null,pendingCollectionIntent:null,setPendingSupplyIntent:()=>true,registerOrganizationSwitchGuard:()=>()=>{}};
 state.options.mockReset().mockResolvedValue({titles:[],reason:"未启用"});
 state.stages.mockReset().mockResolvedValue({items:[],total:205,batchTotal:205,counts:{all:205,waiting:205,missing:0,ready:0,review:0,uploaded:0},nextCursor:source});
 state.preparation.mockReset().mockResolvedValue({id:prep,sourceBatchId:prep,sourceRevision:1,name:"完整批次",count:205,revision:1,createdAt:"2026-10-08T00:00:00Z"});
 state.sources.mockReset().mockResolvedValue({items:[],total:205,nextCursor:source});state.history.mockReset().mockResolvedValue({items:[],total:0});state.write.mockReset().mockResolvedValue({operation:{id:source,input:{preparationId:prep,expectedRevision:1,storeId:store,action:"adapt"},count:205,completed:0,status:"pending",execution:"started",createdAt:"2026-10-08T00:00:00Z"},replayed:false});
 state.stores.mockReset().mockResolvedValue({items:[{id:store,name:"美国试用店铺",platform:"shein",recordStatus:"active",connectionStatus:"connected",serviceStatus:"active"}],pagination:{page:1,pageSize:100,total:1}});
});afterEach(cleanup);
it("filters and counts the complete batch through the projection rather than the visible page",async()=>{
 render(<SupplyPage initialPreparation={prep}/>);await screen.findByText("完整批次");
 await userEvent.selectOptions(await screen.findByLabelText("目标店铺"),store);
 await screen.findByRole("tab",{name:"待适配 (205)"});
 state.stages.mockResolvedValue({items:[],total:0,batchTotal:205,counts:{all:205,waiting:205,missing:0,ready:0,review:0,uploaded:0}});
 await userEvent.click(screen.getByRole("tab",{name:"待审核 (0)"}));
 await waitFor(()=>expect(state.stages).toHaveBeenLastCalledWith({userId:"actor-a",organizationId:"org-a"},prep,store,"review",expect.objectContaining({limit:20}),expect.any(AbortSignal)));
});
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

it("opens the requested Figma waiting stage and disables automatic adaptation when its rule is off",async()=>{
 render(<SupplyPage initialPreparation={prep} initialStage="waiting"/>);await screen.findByText("完整批次");await userEvent.selectOptions(screen.getByLabelText("目标店铺"),store);
 await waitFor(()=>expect(state.stages).toHaveBeenLastCalledWith(expect.anything(),prep,store,"waiting",expect.anything(),expect.any(AbortSignal)));
 expect(screen.getByRole("heading",{name:"待适配"})).toBeInTheDocument();expect(screen.getByRole("button",{name:"开始适配"})).toBeEnabled();
 await userEvent.click(screen.getByRole("switch",{name:"启用本次规则"}));expect(screen.getByRole("button",{name:"开始适配"})).toBeDisabled();expect(state.write).not.toHaveBeenCalled();
});
it("shows the requested review list without requiring the publishing configuration panel",async()=>{
 render(<SupplyPage initialPreparation={prep} initialStage="review"/>);await screen.findByText("完整批次");await userEvent.selectOptions(screen.getByLabelText("目标店铺"),store);
 await waitFor(()=>expect(state.stages).toHaveBeenLastCalledWith(expect.anything(),prep,store,"review",expect.anything(),expect.any(AbortSignal)));
 expect(screen.getByRole("heading",{name:"待审核"})).toBeInTheDocument();expect(screen.queryByText("发布配置流程")).not.toBeInTheDocument();
});

it("navigates dedicated stage tabs with the current batch and store",async()=>{
 render(<SupplyPage initialPreparation={prep} initialStore={store} initialStage="review"/>);await screen.findByText("完整批次");await userEvent.click(screen.getByRole("tab",{name:/已适配/}));expect(state.push).toHaveBeenCalledWith(`/workbench/supply/mine/ready?preparation=${prep}&store=${store}`);
});

it("opens the batch review dialog with the selected actual review items",async()=>{
 state.context.permissions=[...(state.context.permissions as string[]),"listingkit.admin.write"];
 const review={sourceId:source,recordId:prep,recordRevision:1,status:"review",resultReference:store};
 state.stages.mockResolvedValue({items:[{sourceId:source,stage:"review",review}],total:1,batchTotal:1,counts:{all:1,waiting:0,missing:0,ready:0,review:1,uploaded:0}});
 state.detail.mockResolvedValue({source:{id:source,source:{kind:"own",productKey:"product",version:1}},product:{title:"待审核商品"},images:[]});
 state.target.mockResolvedValue({revision:1,result:{issues:[],ready_for_upload:false}});
 state.publication.mockResolvedValue({sourceId:source,storeId:store});
 render(<SupplyPage initialPreparation={prep} initialStore={store} initialStage="review"/>);
 await userEvent.click(await screen.findByLabelText("选择 待审核商品"));
 await userEvent.click(screen.getByRole("button",{name:"批量审核通过"}));
 expect(await screen.findByRole("dialog",{name:"批量审核通过"})).toBeInTheDocument();
 expect(screen.getByText("实际批量提案 1 件")).toBeInTheDocument();expect(state.write).not.toHaveBeenCalled();
});
it("reloads the full-batch projection when the source filter changes",async()=>{
 render(<SupplyPage initialPreparation={prep} initialStore={store}/>);await screen.findByText("完整批次");
 await userEvent.selectOptions(screen.getByLabelText("商品来源"),"acquisition");
 await waitFor(()=>expect(state.stages).toHaveBeenLastCalledWith(expect.anything(),prep,store,"all",expect.objectContaining({sourceKind:"acquisition"}),expect.any(AbortSignal)));
});

it("retains confirmed UNKNOWN readback across progress refresh while other items remain pending verification",async()=>{
 const operation="44444444-4444-4444-8444-444444444444",attempt="55555555-5555-4555-8555-555555555555";
 const op={id:operation,input:{preparationId:prep,expectedRevision:1,storeId:store,action:"upload"},count:2,completed:2,status:"completed",execution:"started",createdAt:"2026-10-08T00:00:00Z"};
 const original={sourceId:source,recordId:prep,recordRevision:1,status:"unknown",resultReference:attempt,note:"原请求响应丢失"};
 const other={sourceId:store,recordId:source,recordRevision:1,status:"unknown",resultReference:operation};
 const product={spu_name:"confirmed-spu",skc_list:[{skc_name:"confirmed-skc",sku_list:[{sku_code:"confirmed-sku",supplier_sku:"sku-a"}]}]};
 state.operation.mockResolvedValue(op);state.history.mockResolvedValue({items:[op],total:1});state.items.mockResolvedValue({items:[original,other],total:2});
 state.attempt.mockResolvedValue({recordId:prep,attemptId:attempt,effectKind:"publish",status:"outcome_unknown",message:"请核实原商品"});
 state.write.mockImplementation(async()=>{state.items.mockResolvedValue({items:[{...original,confirmedProduct:product},other],total:2});return {recordId:prep,publishedRecordId:prep,attemptId:attempt,status:"succeeded",currentRecordUploaded:true,product,message:"原上传已核实"}});
 const view=render(<SupplyPage initialPreparation={prep} initialStore={store} initialOperation={operation}/>);
 const buttons=await screen.findAllByRole("button",{name:"查看待核实结果"});await userEvent.click(buttons[0]);
 await userEvent.type(await screen.findByLabelText("原商品 SPU"),"confirmed-spu");await userEvent.click(screen.getByRole("button",{name:"从官方查询并核实"}));
 expect(await screen.findByText("原执行 UNKNOWN，已核实成功")).toBeInTheDocument();expect(screen.getByText("SPU：confirmed-spu")).toBeInTheDocument();expect(screen.getAllByRole("button",{name:"查看待核实结果"})).toHaveLength(1);
 view.unmount();render(<SupplyPage initialPreparation={prep} initialStore={store} initialOperation={operation}/>);
 expect(await screen.findByText("原执行 UNKNOWN，已核实成功")).toBeInTheDocument();expect(screen.getByText("SPU：confirmed-spu")).toBeInTheDocument();expect(screen.getAllByRole("button",{name:"查看待核实结果"})).toHaveLength(1);expect(state.write).toHaveBeenCalledOnce();
});
