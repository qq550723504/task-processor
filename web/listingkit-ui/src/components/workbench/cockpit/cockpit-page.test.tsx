import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CockpitPage } from "./cockpit-page";
const id = "123e4567-e89b-42d3-a456-426614174000";
const context = vi.hoisted(() => ({ user: { id: "actor-a" }, effectiveOrganization: { id: "org-a" }, operationsCockpitAvailable: true, permissions: ["workbench.cockpit.goals.read"], isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null, retry: vi.fn(), registerOrganizationSwitchGuard: () => () => {} }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => context }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
const capabilities = { access: { goalsRead: true, goalsCreate: true, goalsManage: false, storesRead: false, factsWrite: false, alertsRead: false, adviceRead: false }, today: "2026-10-10", stores: [{ id, name: "真实店铺", platform: "shein", region: "US", status: "active" }] };
const emptyGoal = { goal: null, evaluation: null, head: null, goalUnavailable: false, capturedAt: "2026-10-10T01:00:00Z", basis: [] };
function mount() { return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><CockpitPage mode="settings" /></QueryClientProvider>); }
beforeEach(() => { sessionStorage.clear(); context.effectiveOrganization = { id: "org-a" }; context.operationsCockpitAvailable = true;context.permissions=["workbench.cockpit.goals.read"]; });
afterEach(() => { cleanup(); vi.unstubAllGlobals();vi.restoreAllMocks(); });
it.each([
 ["critical", "异常"],
 ["attention", "需关注"],
 ["data", "数据不完整"],
])("preserves %s severity when selecting advice", async (level, label) => {
 context.permissions = ["workbench.cockpit.advice.read"];
 const rule = { id: "goal:1", kind: "goal_assessment", level, title: "目标进度偏低", reason: "按已完成期间核查经营目标", advice: "核查本期收入与成本", actionPath: "/workbench/overview/goals", actionLabel: "核查经营目标", source: "manual", capturedAt: "2026-10-10T01:00:00Z", period: { startDate: "2026-10-03", endDate: "2026-10-09" } };
 const rules = { rules: [rule], total: 1, page: 1, capturedAt: rule.capturedAt, goalUnavailable: false, observations: { state: "unavailable", exceptional: 0, complete: false, sources: [], latest: [], actionPath: "/workbench/store-orders" } };
 vi.stubGlobal("fetch", vi.fn(async (url: string) => Response.json(url.endsWith("capabilities") ? { ...capabilities, access: { ...capabilities.access, adviceRead: true } } : rules)));
 render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><CockpitPage mode="advice" /></QueryClientProvider>);
 const queuedAdvice = await screen.findByRole("button", { name: /目标进度偏低/ });
 expect(within(queuedAdvice).getByText(label)).toBeVisible();
 fireEvent.click(queuedAdvice);
 expect(screen.getByText(rule.reason)).toBeVisible();
 expect(screen.getByRole("link", { name: "核查经营目标 →" })).toHaveAttribute("href", rule.actionPath);
});
it("retains the identical original operation after an unknown save result", async () => {
 const writes: { body: string; key: string }[] = [];
 vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
  if (init.method === "POST") {
   writes.push({ body: String(init.body), key: new Headers(init.headers).get("Idempotency-Key")! });
   if (writes.length === 1) return Response.json({ code: "OUTCOME_UNKNOWN" }, { status: 502 });
   const body = JSON.parse(String(init.body)); return Response.json({ commandId: writes[0].key, operation: "goal_create", id: body.id, revision: "1", committedAt: "2026-10-10T01:00:00Z" });
  }
  return Response.json(url.endsWith("capabilities") ? capabilities : emptyGoal);
 }));
 mount();await screen.findByText("目标配置");fireEvent.click(screen.getByLabelText("真实店铺 · US"));fireEvent.change(screen.getByLabelText("目标净利润（人民币元）"), { target: { value: "100.25" } });
 await waitFor(() => expect(screen.getByRole("button", { name: "保存并启用" })).toBeEnabled());
 fireEvent.click(screen.getByRole("button", { name: "保存并启用" }));
 await screen.findByRole("button", { name: "重试原操作" });
 fireEvent.click(screen.getByRole("button", { name: "重试原操作" }));
 await screen.findByText("已保存，当前版本 V1。");
 expect(writes).toHaveLength(2);expect(writes[1]).toEqual(writes[0]);
 expect(JSON.parse(writes[0].body).goal.profit).toBe(10025);
 expect(sessionStorage.length).toBe(0);
});
it("does not advertise an unmounted cockpit or fetch its data", async () => {
 vi.stubGlobal("fetch", vi.fn(async () => Response.json(capabilities)));
 context.operationsCockpitAvailable = false;mount();
 expect(await screen.findByText("当前实例尚未开放运营驾驶舱")).toBeVisible();expect(fetch).not.toHaveBeenCalled();
});

it("discards the previous enterprise form draft when the organization changes",async()=>{
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>Response.json(url.endsWith("capabilities")?capabilities:emptyGoal)));
 const client=new QueryClient({defaultOptions:{queries:{retry:false}}});
 const ui=()=> <QueryClientProvider client={client}><CockpitPage mode="settings" /></QueryClientProvider>;
 const view=render(ui());await screen.findByText("目标配置");fireEvent.change(screen.getByLabelText("目标净利润（人民币元）"),{target:{value:"999"}});
 context.effectiveOrganization={id:"org-b"};view.rerender(ui());await screen.findByText("目标配置");expect(screen.getByLabelText("目标净利润（人民币元）")).toHaveValue("");
});

it("starts a fresh record after retrying an unknown financial save",async()=>{
 context.permissions=["workbench.cockpit.stores.read"];
 const caps={...capabilities,access:{...capabilities.access,storesRead:true,factsWrite:true}}, totals={revenue:0,refunds:0,procurement:0,logistics:0,platform:0,advertising:0,other:0,netRevenue:0,netProfit:0,margin:null},period={startDate:"2026-10-03",endDate:"2026-10-09"},stamp="2026-10-10T01:00:00Z",writes:{body:string,key:string}[]=[];
 vi.stubGlobal("fetch",vi.fn(async(url:string,init:RequestInit)=>{
  if(init.method==="POST") {writes.push({body:String(init.body),key:new Headers(init.headers).get("Idempotency-Key")!});if(writes.length===1)return Response.json({code:"OUTCOME_UNKNOWN"},{status:502});const body=JSON.parse(String(init.body));return Response.json({commandId:writes.at(-1)!.key,operation:"fact_create",id:body.id,revision:"1",committedAt:stamp});}
  if(url.endsWith("capabilities"))return Response.json(caps);
  if(url.includes("/facts?"))return Response.json([]);
  if(url.includes(`/stores/${id}?`))return Response.json({store:capabilities.stores[0],aggregate:{storeId:id,period,complete:false,totals,gaps:[period],records:[],excluded:[]},growth:null,capturedAt:stamp});
  return Response.json({rows:[{store:capabilities.stores[0],complete:false,state:"data_incomplete",totals,growth:null,gapCount:1,excludedCount:0,recordCount:0}],total:1,page:1,summary:totals,summaryComplete:false,states:{loss:0,break_even:0,recorded_complete:0,data_incomplete:1},capturedAt:stamp});
 }));
 render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><CockpitPage mode="stores" /></QueryClientProvider>);
 fireEvent.click(await screen.findByRole("button",{name:"查看 / 录入"}));fireEvent.click(await screen.findByRole("button",{name:"录入经营数据"}));
 for(const label of ["销售收入（退款前）","退款","本期已销售商品采购成本","物流成本","平台费用","广告成本","其他成本"])fireEvent.change(screen.getByLabelText(label),{target:{value:"0"}});
 fireEvent.click(screen.getByRole("button",{name:"保存经营数据"}));fireEvent.click(await screen.findByRole("button",{name:"重试原操作"}));await screen.findByText("已保存，当前版本 V1。");
 expect(writes[1]).toEqual(writes[0]);expect(screen.queryByRole("button",{name:"保存经营数据"})).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"录入经营数据"}));for(const label of ["销售收入（退款前）","退款","本期已销售商品采购成本","物流成本","平台费用","广告成本","其他成本"])fireEvent.change(screen.getByLabelText(label),{target:{value:"0"}});fireEvent.click(screen.getByRole("button",{name:"保存经营数据"}));await waitFor(()=>expect(writes).toHaveLength(3));expect(JSON.parse(writes[2].body).id).not.toBe(JSON.parse(writes[0].body).id);
});

it("keeps unknown old-scope evidence while explicitly reconfiguring the current goal",async()=>{
 const writes:{body:string,key:string}[]=[];let lost=false;const newStore="123e4567-e89b-42d3-a456-426614174002";
 const head={goalId:id,revision:"1",scopeValid:false,canReconfigure:true};
 vi.stubGlobal("fetch",vi.fn(async(url:string,init:RequestInit)=>{
  if(init.method==="POST"){
   writes.push({body:String(init.body),key:new Headers(init.headers).get("Idempotency-Key")!});
   if(writes.length===1){lost=true;return Response.json({code:"OUTCOME_UNKNOWN"},{status:502});}
   if(writes.length===2)return Response.json({code:"FORBIDDEN"},{status:403});
   const body=JSON.parse(String(init.body));return Response.json({commandId:writes[2].key,operation:"goal_update",id:body.id,revision:"2",committedAt:"2026-10-10T01:00:00Z"});
  }
  if(url.endsWith("capabilities"))return Response.json(lost?{...capabilities,stores:[{...capabilities.stores[0],id:newStore,name:"现授权店铺"}]}:capabilities);
  return Response.json(lost?{...emptyGoal,goalUnavailable:true,head}:emptyGoal);
 }));
 // Fix the goal ID so the authoritative head can identify the original unknown request.
 const random=vi.spyOn(crypto,"randomUUID").mockReturnValue(id);
 mount();await screen.findByText("目标配置");fireEvent.click(screen.getByLabelText("真实店铺 · US"));fireEvent.change(screen.getByLabelText("目标净利润（人民币元）"),{target:{value:"100"}});await waitFor(()=>expect(screen.getByRole("button",{name:"保存并启用"})).toBeEnabled());fireEvent.click(screen.getByRole("button",{name:"保存并启用"}));
 fireEvent.click(await screen.findByRole("button",{name:"重试原操作"}));
 fireEvent.click(await screen.findByRole("button",{name:"保留原操作并维护当前版本"}));
 fireEvent.click(await screen.findByLabelText("现授权店铺 · US"));fireEvent.change(screen.getByLabelText("目标净利润（人民币元）"),{target:{value:"200"}});random.mockRestore();fireEvent.click(screen.getByRole("button",{name:"保存并启用"}));
 await screen.findByText("已保存，当前版本 V2。");expect(writes[1]).toEqual(writes[0]);expect(JSON.parse(writes[2].body)).toMatchObject({id,expectedRevision:"1",goal:{profit:20000,storeIds:[newStore]}});expect(writes[2].key).not.toBe(writes[0].key);expect(sessionStorage.length).toBe(1);expect(sessionStorage.getItem(sessionStorage.key(0)!)).toContain(writes[0].key);
});
