import {act,cleanup,fireEvent,render,renderHook,screen,waitFor} from "@testing-library/react";
import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import type {ReactNode} from "react";
import {ecoPolicy,ecoRequest,EcoservicesError,type EcoRequest} from "@/lib/api/ecoservices";
import {RequestDetail} from "./request-detail";
import {EcoservicesJoin} from "./join";
import {useEcoCommands} from "./shared";

const context=vi.hoisted(()=>({user:{id:"actor"},effectiveOrganization:{id:"org"},permissions:["workbench.ecoservices.purchase"],isLoading:false,isSwitching:false,error:null,blockingError:null,registerOrganizationSwitchGuard:vi.fn(()=>()=>undefined)}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>context}));
vi.mock("@/components/workbench/resources/resource-dialog",()=>({ResourceDialog:({children}:{children:ReactNode})=><div>{children}</div>}));
vi.mock("@/lib/api/ecoservices",async original=>({...await original<typeof import("@/lib/api/ecoservices")>(),ecoRequest:vi.fn()}));
beforeEach(()=>{localStorage.clear();Object.defineProperty(navigator,"locks",{configurable:true,value:{request:async(_name:string,_options:unknown,run:(lock:unknown)=>Promise<unknown>)=>run({})}})});
afterEach(()=>{cleanup();localStorage.clear();vi.clearAllMocks();context.isSwitching=false;context.permissions=["workbench.ecoservices.purchase"]});
const scope={userId:"actor",organizationId:"org"},id="4841d296-ef14-4c16-8d25-a7667e534feb";
function request():EcoRequest{return {id,listingId:id,listingVersion:"1",title:"原服务",category:"STORE_OPENING",description:"原需求",fileIds:[],state:"AWAITING_ACCEPTANCE",version:"8",quote:{commissionBps:1000,allocationBasis:"CUMULATIVE_NET_FLOOR_V1",policyVersion:ecoPolicy,amountMinor:"10000",scope:"交付店铺",acceptanceCriteria:"可登录",deliveryDays:7,version:"1"},delivery:{content:"原交付",fileIds:[],version:"1",submittedAt:"2026-10-08T00:00:00Z"},acceptedDeliveryVersion:"0",financialHold:false,financialState:"PAID",financialReason:"",createdAt:"2026-10-08T00:00:00Z",updatedAt:"2026-10-08T00:00:00Z",side:"buyer"}}
function harness(){const client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});const wrapper=({children}:{children:ReactNode})=><QueryClientProvider client={client}>{children}</QueryClientProvider>;return {client,wrapper}}
it("limits the reviewed original license upload to channel compatible images",async()=>{
 const {client,wrapper}=harness();context.permissions=["workbench.ecoservices.join"];vi.mocked(ecoRequest).mockResolvedValue({total:"0"});render(<EcoservicesJoin/>,{wrapper});
 const open=screen.getByRole("button",{name:"提交机构入驻申请 →"});await waitFor(()=>expect(open).toBeEnabled());fireEvent.click(open);
 const input=screen.getByLabelText(/私有材料/);expect(input).toHaveAttribute("accept",".png,.jpg,.jpeg");
 fireEvent.change(input,{target:{files:[new File([new Uint8Array(2*1024*1024+1)],"oversized.png",{type:"image/png"})]}});
 expect(await screen.findAllByText("每份材料不能超过2 MiB")).not.toHaveLength(0);expect(vi.mocked(ecoRequest).mock.calls.every(v=>!v[3]?.method)).toBe(true);client.clear();
});
it("lets a rejected provider correct the original version and returns to review",async()=>{
 const {client,wrapper}=harness();context.permissions=["workbench.ecoservices.join"];
 const rejected={id,companyName:"原机构",registrationNumber:"original-registration",categories:["COMPANY_REGISTRATION"],regions:["中国"],fileIds:[id],state:"REJECTED",version:"2",agreementVersion:ecoPolicy,agreementAccepted:false,onboardingState:"NOT_STARTED",reviewReason:"请更正登记号",updatedAt:"2026-10-08T00:00:00Z"};
 let submitted=false;vi.mocked(ecoRequest).mockImplementation(async(_scope,_path,_schema,init)=>{if(init?.method){submitted=true;return {application:{...rejected,registrationNumber:"corrected-registration",state:"SUBMITTED",version:"3",reviewReason:""}}}return {applications:[submitted?{...rejected,state:"SUBMITTED",version:"3",reviewReason:""}:rejected],total:"1"}});
 render(<EcoservicesJoin/>,{wrapper});const open=await screen.findByRole("button",{name:"修正资料并重新提交 →"});expect(open).toBeEnabled();fireEvent.click(open);
 expect(screen.getByLabelText("机构法定名称")).toHaveValue("原机构");fireEvent.change(screen.getByLabelText("统一社会信用代码或登记号"),{target:{value:"corrected-registration"}});fireEvent.click(screen.getByRole("button",{name:"提交更正版本"}));
 await waitFor(()=>expect(vi.mocked(ecoRequest).mock.calls.some(v=>v[3]?.method==="POST")).toBe(true));const sent=vi.mocked(ecoRequest).mock.calls.find(v=>v[3]?.method==="POST")!;expect(sent[1]).toBe("applications");expect(new Headers(sent[3]!.headers).get("If-Match")).toBe('"2"');expect(JSON.parse(sent[3]!.body as string)).toMatchObject({registrationNumber:"corrected-registration",fileIds:[id]});
 await waitFor(()=>expect(screen.queryByRole("button",{name:"提交更正版本"})).not.toBeInTheDocument());expect(screen.getByRole("button",{name:"提交机构入驻申请 →"})).toBeDisabled();client.clear();
});
it("shows the durable rejection reason to the correcting provider",async()=>{
 const {client,wrapper}=harness(),r=request();context.permissions=["workbench.ecoservices.manage"];
 const corrected={...r,state:"SERVICING",side:"provider",delivery:{...r.delivery!,rejection:{deliveryVersion:"1",reason:"缺少注册证明",actorId:"buyer-user",rejectedAt:"2026-10-08T01:00:00Z"}}};
 vi.mocked(ecoRequest).mockResolvedValue({requests:[corrected],total:"1"});render(<RequestDetail scope={scope} id={id} onClose={()=>undefined}/>,{wrapper});
 expect(await screen.findByText("缺少注册证明")).toBeVisible();expect(screen.getByText(/客户拒收交付版本 1/)).toBeVisible();client.clear();
});
it("requires fresh customer consent when the original delivery version changes",async()=>{
 const {client,wrapper}=harness(),r=request();vi.mocked(ecoRequest).mockResolvedValue({requests:[r],total:"1"});render(<RequestDetail scope={scope} id={id} onClose={()=>undefined}/>,{wrapper});
 const consent=await screen.findByRole("checkbox",{name:/已检查并确认交付版本 1/});fireEvent.click(consent);expect(screen.getByRole("button",{name:"确认验收并结算"})).toBeEnabled();
 act(()=>client.setQueryData(["ecoservices",scope.userId,scope.organizationId,"request",id,false,context.permissions.join("|")],{requests:[{...r,version:"9",delivery:{...r.delivery!,version:"2"}}],total:"1"}));
 await waitFor(()=>expect(screen.getByRole("checkbox",{name:/已检查并确认交付版本 2/})).not.toBeChecked());expect(screen.getByRole("button",{name:"确认验收并结算"})).toBeDisabled();expect(vi.mocked(ecoRequest).mock.calls.every(v=>!v[3]?.method)).toBe(true);client.clear();
});
it("keeps the same key, exact body and CAS after an unknown write and refuses a new intent",async()=>{
 const {client,wrapper}=harness();vi.mocked(ecoRequest).mockRejectedValueOnce(new EcoservicesError("OUTCOME_UNKNOWN"));const {result}=renderHook(()=>useEcoCommands(scope),{wrapper});
 await act(async()=>{await result.current.json("requests/"+id+"/accept",{deliveryVersion:"3"},"7").catch(()=>undefined)});
 const original=result.current.intent!;expect(original).toBeTruthy();const sent=vi.mocked(ecoRequest).mock.calls[0][3]!;const key=new Headers(sent.headers).get("Idempotency-Key");
 await expect(result.current.json("requests/"+id+"/accept",{deliveryVersion:"4"},"8")).rejects.toMatchObject({code:"ECOSERVICES_CONFLICT"});expect(ecoRequest).toHaveBeenCalledTimes(1);
 vi.mocked(ecoRequest).mockResolvedValueOnce({request:request()});await act(async()=>{await result.current.execute(original)});
 const retried=vi.mocked(ecoRequest).mock.calls[1][3]!;expect(new Headers(retried.headers).get("Idempotency-Key")).toBe(key);expect(new Headers(retried.headers).get("If-Match")).toBe('"7"');expect(retried.body).toBe(sent.body);expect(result.current.intent).toBeNull();client.clear();
});
