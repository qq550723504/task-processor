import {afterEach,expect,it,vi} from "vitest";
import {render,screen,fireEvent,cleanup} from "@testing-library/react";
import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {MerchantOnboarding} from "./merchant";
import {ecoRequest,type EcoApplication} from "@/lib/api/ecoservices";
const mocks=vi.hoisted(()=>({execute:vi.fn().mockResolvedValue({}),locked:false}));
vi.mock("@/lib/api/ecoservices",async importOriginal=>({...await importOriginal<typeof import("@/lib/api/ecoservices")>(),ecoRequest:vi.fn()}));
vi.mock("./shared",()=>({useEcoCommands:()=>({...mocks,notice:null}),ReadFailure:()=>null,date:()=>"today",yuan:(v:string)=>v}));
vi.mock("./files",()=>({FileUpload:()=>null}));
vi.mock("@/components/workbench/resources/resource-dialog",()=>({ResourceDialog:({children}:{children:React.ReactNode})=><div>{children}</div>}));
const id="4841d296-ef14-4c16-8d25-a7667e534feb";
const application:EcoApplication={id,companyName:"原企业",registrationNumber:"original-registration",categories:["COMPANY_REGISTRATION"],regions:["China"],fileIds:[id],state:"APPROVED",version:"8",agreementVersion:"current",agreementAccepted:true,onboardingState:"REJECTED",reviewReason:"",updatedAt:"2026-10-08T00:00:00Z"};
function show(pending=false,version="2",state="REJECTED"){vi.mocked(ecoRequest).mockResolvedValue({verificationPending:pending,revisionVersion:version,canCorrect:!pending,id,applicationId:id,state,signState:"",signUrl:"",legalValidationUrl:"",reason:"修正银行资料",updatedAt:application.updatedAt});const client=new QueryClient({defaultOptions:{queries:{retry:false}}});return render(<QueryClientProvider client={client}><MerchantOnboarding scope={{userId:"actor",organizationId:"org"}} application={application}/></QueryClientProvider>)}
afterEach(()=>{cleanup();vi.clearAllMocks()});
it("opens correction and sends the precise current details and application versions",async()=>{
 const view=show();fireEvent.click(await screen.findByRole("button",{name:"更正微信商户资料"}));
 fireEvent.submit(view.container.querySelector("form")!);
 expect(mocks.execute).toHaveBeenCalledTimes(1);const command=mocks.execute.mock.calls[0][0];expect(command.version).toBe("8");expect(JSON.parse(command.body).expectedRevisionVersion).toBe("2");expect(command.path).toBe("applications/"+id+"/merchant");
});
it("keeps an unverified correction unavailable for changed-payload submission",async()=>{
 show(true);await screen.findByText(/修正银行资料/);expect(screen.queryByRole("button",{name:"更正微信商户资料"})).not.toBeInTheDocument();
});

it("preserves original v1 resume after unknown or verified absence, but hides unacknowledged correction resume",async()=>{
 show(true,"1","PREPARING");const resume=await screen.findByRole("button",{name:"继续原商户申请"});fireEvent.click(resume);expect(mocks.execute.mock.calls[0][0].path).toBe("applications/"+id+"/merchant/resume");
 cleanup();show(true,"2","PREPARING");await screen.findByText(/修正银行资料/);expect(screen.queryByRole("button",{name:"继续原商户申请"})).not.toBeInTheDocument();
});
