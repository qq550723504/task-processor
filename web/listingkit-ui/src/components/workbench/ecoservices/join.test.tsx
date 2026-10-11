import {afterEach,expect,it,vi} from "vitest";
import {render,screen,cleanup,fireEvent,waitFor,act} from "@testing-library/react";
import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {EcoservicesJoin} from "./join";
import {ecoRequest} from "@/lib/api/ecoservices";
vi.mock("@/lib/api/ecoservices",async importOriginal=>({...await importOriginal<typeof import("@/lib/api/ecoservices")>(),ecoRequest:vi.fn()}));
const state=vi.hoisted(()=>({permissions:["workbench.ecoservices.join"],json:vi.fn().mockResolvedValue({})}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>({permissions:state.permissions})}));
vi.mock("./shared",async original=>({...await original<typeof import("./shared")>(),EcoBoundary:({children}:{children:(v:unknown)=>React.ReactNode})=>children({userId:"actor",organizationId:"org"}),useEcoCommands:()=>({notice:null,locked:false,json:state.json}),ReadFailure:()=> <p>读取失败</p>,EcoPagination:()=>null,date:()=>"today"}));
vi.mock("@/components/workbench/resources/resource-dialog",()=>({ResourceDialog:({children}:{children:React.ReactNode})=><div>{children}</div>}));
vi.mock("./files",()=>({FileLinks:()=>null,FileUpload:()=>null}));
vi.mock("./merchant",()=>({MerchantOnboarding:()=> <p>商户执行入口</p>}));
vi.mock("@/components/workbench/console/console-page",()=>({ConsolePage:({children}:{children:React.ReactNode})=><div>{children}</div>,ConsoleState:()=>null}));
afterEach(()=>{cleanup();vi.clearAllMocks();state.permissions=["workbench.ecoservices.join"]});
it("allows original business rejection correction and marks the unopened channel re-review rejection unavailable",async()=>{
 const id="4841d296-ef14-4c16-8d25-a7667e534feb";
 const app={id,companyName:"原企业",registrationNumber:"registration",categories:["COMPANY_REGISTRATION"],regions:["China"],fileIds:[id],state:"REJECTED",version:"8",agreementVersion:"current",agreementAccepted:false,onboardingState:"NOT_STARTED",reviewReason:"reviewed",updatedAt:"2026-10-08T00:00:00Z"};
 const show=(onboardingState:string)=>{vi.mocked(ecoRequest).mockResolvedValue({applications:[{...app,onboardingState}],total:"1"});return render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><EcoservicesJoin/></QueryClientProvider>)};
 show("NOT_STARTED");expect(await screen.findByRole("button",{name:"修正资料并重新提交 →"})).toBeEnabled();cleanup();
 show("PREPARING");await screen.findByText("原入驻申请");expect(screen.getByRole("button",{name:/修正资料并重新提交|提交机构入驻申请/})).toBeDisabled();
});
const id="4841d296-ef14-4c16-8d25-a7667e534feb";
const listing={id,providerName:"原企业",category:"COMPANY_REGISTRATION",title:"原服务",description:"原服务说明",items:["企业登记"],regions:["上海"],platforms:[],priceMinor:"101",deliveryDays:7,state:"DRAFT",version:"3"};
function renderOperator(page:unknown){state.permissions=["workbench.ecoservices.read","workbench.ecoservices.manage"];vi.mocked(ecoRequest).mockImplementation(async(_scope,path)=>{if(!path.startsWith("provider/listings?"))throw new Error("operator requested join-only data");return page});const client=new QueryClient({defaultOptions:{queries:{retry:false}}});return {...render(<QueryClientProvider client={client}><EcoservicesJoin/></QueryClientProvider>),client}}
it("lets a manage-only operator create, edit and publish using the original managed listing commands",async()=>{
 renderOperator({providerQualified:true,listings:[listing],total:"1"});
 await screen.findByText("原服务");
 expect(await screen.findByRole("button",{name:"新增服务"})).toBeEnabled();
 fireEvent.click(screen.getByRole("button",{name:"发布"}));await waitFor(()=>expect(state.json).toHaveBeenCalledWith("provider/listings/"+id+"/publish",{},"3"));
 fireEvent.click(screen.getByRole("button",{name:"编辑"}));expect(screen.getByLabelText("服务名称")).toHaveValue("原服务");fireEvent.change(screen.getByLabelText("服务名称"),{target:{value:"更正服务"}});fireEvent.click(screen.getByRole("button",{name:"保存服务草稿"}));
 await waitFor(()=>expect(state.json).toHaveBeenCalledWith("provider/listings/"+id,expect.objectContaining({title:"更正服务",priceMinor:"101"}),"3",false,"PUT"));
 await waitFor(()=>expect(screen.queryByLabelText("服务名称")).not.toBeInTheDocument());
 fireEvent.click(screen.getByRole("button",{name:"新增服务"}));
 for(const [label,value] of [["服务名称","新增注册服务"],["服务描述","原企业新服务"],["包含服务（每行一项，最多10项）","企业登记"],["服务地区（每行一项）","上海"],["参考价（元）","2.01"]])fireEvent.change(screen.getByLabelText(label),{target:{value}});
 fireEvent.click(screen.getByRole("button",{name:"保存服务草稿"}));await waitFor(()=>expect(state.json).toHaveBeenCalledWith("provider/listings",expect.objectContaining({title:"新增注册服务",priceMinor:"201"}),undefined,false,"POST"));
 expect(vi.mocked(ecoRequest).mock.calls.every(v=>v[1].startsWith("provider/listings?"))).toBe(true);
});
it.each(["qualification","permission"])("fences an already opened listing form when its current %s changes",async(change)=>{
 const h=renderOperator({providerQualified:true,listings:[listing],total:"1"});await screen.findByText("原服务");fireEvent.click(screen.getByRole("button",{name:"编辑"}));expect(screen.getByLabelText("服务名称")).toBeEnabled();
 if(change==="qualification")act(()=>h.client.setQueryData(["ecoservices","actor","org","provider-listings",1,state.permissions.join("|")],{providerQualified:false,listings:[listing],total:"1"}));
 else {state.permissions=["workbench.ecoservices.read"];h.rerender(<QueryClientProvider client={h.client}><EcoservicesJoin/></QueryClientProvider>)}
 await waitFor(()=>expect(screen.getByLabelText("服务名称")).toBeDisabled());fireEvent.submit(screen.getByLabelText("服务名称").closest("form")!);expect(state.json).not.toHaveBeenCalled();
});
it.each([false,undefined])("does not enable listing mutations when current manage qualification is %s",async(providerQualified)=>{
 renderOperator({providerQualified,listings:[listing],total:"1"});await screen.findByText("原服务");
 for(const name of ["新增服务","编辑","发布","发布专业服务 →"])expect(screen.getByRole("button",{name})).toBeDisabled();expect(state.json).not.toHaveBeenCalled();
});
it("keeps management fail-closed when its query fails even if the join-only application is active",async()=>{
 state.permissions=["workbench.ecoservices.join","workbench.ecoservices.manage"];vi.mocked(ecoRequest).mockImplementation(async(_scope,path)=>{if(path.startsWith("provider/listings?"))throw new Error("unavailable");return {applications:[{id,companyName:"原企业",registrationNumber:"original",categories:[],regions:[],fileIds:[],state:"ACTIVE",version:"1",agreementAccepted:true,onboardingState:"FINISH",updatedAt:"2026-10-08T00:00:00Z"}],total:"1"}});
 render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><EcoservicesJoin/></QueryClientProvider>);await screen.findByText("读取失败");
 expect(screen.getByRole("button",{name:"新增服务"})).toBeDisabled();expect(screen.getByRole("button",{name:"发布专业服务 →"})).toBeDisabled();expect(state.json).not.toHaveBeenCalled();
});

it("qualification-only mode permits agreement but hides merchant execution and stale qualified listing controls",async()=>{
 state.permissions=["workbench.ecoservices.join","workbench.ecoservices.manage"];
 vi.mocked(ecoRequest).mockImplementation(async(_scope,path)=>path.startsWith("provider/listings?")?{providerQualified:true,listings:[listing],total:"1"}:{applications:[{id,companyName:"原企业",registrationNumber:"original",categories:[],regions:[],fileIds:[],state:"APPROVED",version:"1",agreementAccepted:false,onboardingState:"NOT_STARTED",updatedAt:"2026-10-08T00:00:00Z"}],total:"1"});
 render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><EcoservicesJoin nonPaymentOnly/></QueryClientProvider>);
 await screen.findByText("原入驻申请");
 expect(screen.queryByText("商户执行入口")).not.toBeInTheDocument();
 expect(screen.getByRole("button",{name:"确认当前协议"})).toBeInTheDocument();
 for(const name of ["新增服务","编辑","发布","发布专业服务 →"])expect(screen.getByRole("button",{name})).toBeDisabled();
 expect(screen.getByText(/当前开放资质申请、平台审核和协议确认/)).toBeInTheDocument();
});
