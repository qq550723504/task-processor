import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CollectionAPIError, type CollectionIntent } from "@/lib/api/product-collection";
import { CollectionPage } from "./collection-page";
import { SupplyAPIError,type SupplyIntent } from "@/lib/api/supply-chain";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, list: vi.fn(), items: vi.fn(), mutate: vi.fn(), read: vi.fn(),supplyWrite:vi.fn(),supplyRead:vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/product-collection", async original => ({ ...await original<object>(), listCollectionBatches: state.list, listCollectionItems:state.items, listOwnProducts:state.items, mutateCollection: state.mutate, readCollectionOperation: state.read }));
vi.mock("@/lib/api/supply-chain",async original=>({...await original<object>(),supplyCommand:state.supplyWrite,readSupplyCommand:state.supplyRead}));
beforeEach(() => {
  state.context = { user: { id: "actor-a" }, effectiveOrganization: { id: "org-a" }, permissions: ["workbench.collection.manage"],
    pendingCollectionIntent: null, collectionIntentReady: true, setPendingCollectionIntent: (intent: CollectionIntent | null) => { state.context.pendingCollectionIntent = intent; return true; }, registerOrganizationSwitchGuard: () => () => {} };
  state.list.mockReset().mockResolvedValue({ items: [], total: 0 }); state.mutate.mockReset(); state.read.mockReset();
  state.items.mockReset().mockResolvedValue({ items: [], total: 0 });
  state.context.pendingSupplyIntent=null;state.context.supplyIntentReady=true;state.context.setPendingSupplyIntent=(intent:SupplyIntent|null)=>{state.context.pendingSupplyIntent=intent;return true};
  state.supplyWrite.mockReset();state.supplyRead.mockReset();
});
afterEach(cleanup);

it("keeps blank SDS template batches out of finished Supply Chain transfer",async()=>{
 state.context.permissions=["workbench.collection.read","workbench.supply.manage"];
 state.list.mockResolvedValue({items:[{id:"550e8400-e29b-41d4-a716-446655440000",name:"模板批次",kind:"sds_template",supplyTransferSupported:false,revision:1,count:1,createdAt:"2026-10-08T00:00:00Z"}],total:1});
 render(<CollectionPage supplyAvailable />);
 expect(await screen.findByRole("button",{name:"加入我的供应链"})).toBeDisabled();
 expect(screen.getByRole("button",{name:"加入我的供应链"})).toHaveAttribute("title","SDS 模板须先定制成品，再加入供应链");
 expect(state.supplyWrite).not.toHaveBeenCalled();
});

it("allows current eligible products moved into a former SDS template batch",async()=>{
 state.context.permissions=["workbench.collection.read","workbench.supply.manage"];
 state.list.mockResolvedValue({items:[{id:"550e8400-e29b-41d4-a716-446655440000",name:"现有成品",kind:"sds_template",supplyTransferSupported:true,revision:2,count:1,createdAt:"2026-10-08T00:00:00Z"}],total:1});
 render(<CollectionPage supplyAvailable />);
 expect(await screen.findByRole("button",{name:"加入我的供应链"})).toBeEnabled();
 expect(screen.getByRole("button",{name:"加入我的供应链"})).not.toHaveAttribute("title");
});

it.each([true,false])("loads paged move targets for a deep link or own-product view (deep=%s)",async deep=>{
 const source="550e8400-e29b-41d4-a716-446655440000",target="550e8400-e29b-41d4-a716-446655440001",item="550e8400-e29b-41d4-a716-446655440002";
 const batch=(id:string,name:string)=>({id,name,kind:"manual",revision:1,count:1,supplyTransferSupported:false,createdAt:"2026-10-10T00:00:00Z"});
 state.items.mockResolvedValue({items:[{id:item,batchId:source,revision:2,title:"saved fixture",source:{kind:"amazon_data",productKey:"fixture"},createdAt:"2026-10-10T00:00:00Z"}],total:1});
 state.list.mockImplementation(async (_scope,query)=>query.after?{items:[batch(target,"目标批次")],total:2}:{items:[batch(source,"原批次")],nextCursor:source,total:2});
 state.mutate.mockResolvedValue({operationId:item,revision:3,replayed:false});
 render(<CollectionPage initialBatchId={deep?source:undefined}/>);
 if(!deep)await userEvent.click(screen.getByRole("tab",{name:"自有商品库"}));
 await userEvent.click(await screen.findByRole("button",{name:"移动批次"}));
 await userEvent.click(await screen.findByRole("button",{name:"下一页批次"}));
 await screen.findByRole("option",{name:"目标批次"});
 await userEvent.selectOptions(screen.getByLabelText("目标批次"),target);
 await userEvent.click(screen.getByRole("button",{name:"确认移动"}));
 await waitFor(()=>expect(state.mutate).toHaveBeenCalledOnce());
 expect(state.mutate.mock.calls[0][0]).toMatchObject({userId:"actor-a",organizationId:"org-a",command:{action:"move_item",itemId:item,targetBatchId:target,expectedRevision:2}});
 expect(state.list.mock.calls.some(([scope,query,signal])=>scope.organizationId==="org-a"&&query.after===source&&signal instanceof AbortSignal)).toBe(true);
});

it("does not dispatch a command when its original recovery intent cannot be retained", async () => {
  state.context.setPendingCollectionIntent = () => false;
  render(<CollectionPage />);
  await userEvent.click(screen.getByRole("button", { name: "管理批次" }));
  await userEvent.type(screen.getByLabelText("新批次名称"), "运营资料");
  await userEvent.click(screen.getByRole("button", { name: "新建批次" }));
  expect(state.mutate).not.toHaveBeenCalled();
});

it("transfers all 205 batch members and recovers an uncertain original key",async()=>{
 const id="550e8400-e29b-41d4-a716-446655440000";
 state.context.permissions=["workbench.collection.read","workbench.supply.manage"];
 state.list.mockResolvedValue({items:[{id,name:"供应测试批次",kind:"manual",revision:1,count:205,supplyTransferSupported:true,createdAt:"2026-10-08T00:00:00Z"}],total:1});
 state.supplyWrite.mockRejectedValue(new SupplyAPIError("OUTCOME_UNKNOWN",503));
 render(<CollectionPage supplyAvailable />);
 await userEvent.click(await screen.findByRole("button",{name:"加入我的供应链"}));
 await userEvent.click(screen.getByRole("button",{name:"确认加入"}));
 await screen.findByText("结果待核实");
 const intent=state.supplyWrite.mock.calls[0][0] as SupplyIntent;
 expect(intent.command).toEqual({batchId:id,expectedRevision:1});expect(intent.route).toBe("transfer");
 state.supplyRead.mockResolvedValue({preparation:{id,sourceBatchId:id,sourceRevision:1,name:"供应测试批次",count:205,revision:1,createdAt:"2026-10-08T00:00:00Z"},replayed:true});
 await userEvent.click(screen.getByRole("button",{name:"核实原操作"}));
 await screen.findByRole("link",{name:"查看我的供应链"});
 expect(state.supplyRead).toHaveBeenCalledWith(intent,expect.any(AbortSignal));expect(state.supplyWrite).toHaveBeenCalledOnce();
});

it.each(["amazon_data", "custom_dataset", "manual"])("hides supply transfer for unsupported sources in a %s batch", async kind => {
 state.context.permissions=["workbench.collection.read","workbench.supply.manage"];
 state.list.mockResolvedValue({items:[{id:"550e8400-e29b-41d4-a716-446655440000",name:"待查看数据",kind,revision:2,count:2,supplyTransferSupported:false,createdAt:"2026-10-08T00:00:00Z"}],total:1});
 render(<CollectionPage supplyAvailable />);
 await screen.findByText("待查看数据");
 expect(screen.queryByRole("button",{name:"加入我的供应链"})).not.toBeInTheDocument();
 expect(screen.getByRole("button",{name:"查看商品"})).toBeEnabled();
 expect(state.supplyWrite).not.toHaveBeenCalled();
});

it("keeps an uncertain batch command and verifies the original key without another write", async () => {
  state.mutate.mockRejectedValue(new CollectionAPIError("OUTCOME_UNKNOWN", 503));
  render(<CollectionPage />);
  await userEvent.click(screen.getByRole("button", { name: "管理批次" }));
  await userEvent.type(screen.getByLabelText("新批次名称"), "运营资料");
  await userEvent.click(screen.getByRole("button", { name: "新建批次" }));
  await screen.findByText("结果待核实");
  const intent = state.mutate.mock.calls[0][0] as CollectionIntent;
  expect(intent).toMatchObject({ userId: "actor-a", organizationId: "org-a", command: { action: "create_batch", name: "运营资料" } });
  state.read.mockResolvedValue({ operationId: "550e8400-e29b-41d4-a716-446655440000", revision: 1, replayed: true });
  await userEvent.click(screen.getByRole("button", { name: "核实原操作" }));
  await waitFor(() => expect(state.read).toHaveBeenCalledWith(intent, expect.any(AbortSignal)));
  expect(state.mutate).toHaveBeenCalledOnce();
});

it("aborts an old enterprise read and ignores its late batch data", async () => {
  let resolve!: (value: unknown) => void;
  state.list.mockReturnValueOnce(new Promise(ok => { resolve = ok; }));
  const view = render(<CollectionPage />);
  await waitFor(() => expect(state.list).toHaveBeenCalledOnce());
  const signal = state.list.mock.calls[0][2] as AbortSignal;
  state.context = { ...state.context, user: { id: "actor-b" }, effectiveOrganization: { id: "org-b" } };
  view.rerender(<CollectionPage />);
  expect(signal.aborted).toBe(true);
  await act(async () => resolve({ items: [{ id: "550e8400-e29b-41d4-a716-446655440000", name: "旧企业私有批次", kind: "manual", revision: 1, count: 1, supplyTransferSupported:true, createdAt: "2026-10-08T00:00:00Z" }], total: 1 }));
  expect(screen.queryByText("旧企业私有批次")).not.toBeInTheDocument();
});
