import {afterEach,expect,it,vi} from "vitest";
import {render,screen,cleanup} from "@testing-library/react";
import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {EcoservicesJoin} from "./join";
import {ecoRequest} from "@/lib/api/ecoservices";
vi.mock("@/lib/api/ecoservices",async importOriginal=>({...await importOriginal<typeof import("@/lib/api/ecoservices")>(),ecoRequest:vi.fn()}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>({permissions:["workbench.ecoservices.join"]})}));
vi.mock("./shared",()=>({EcoBoundary:({children}:{children:(v:unknown)=>React.ReactNode})=>children({userId:"actor",organizationId:"org"}),useEcoCommands:()=>({notice:null,locked:false}),ReadFailure:()=>null,EcoPagination:()=>null,date:()=>"today",categories:[{id:"COMPANY_REGISTRATION",name:"公司注册"}]}));
vi.mock("./files",()=>({FileLinks:()=>null,FileUpload:()=>null}));
vi.mock("@/components/workbench/console/console-page",()=>({ConsolePage:({children}:{children:React.ReactNode})=><div>{children}</div>,ConsoleState:()=>null}));
afterEach(()=>{cleanup();vi.clearAllMocks()});
it("allows original business rejection correction and marks the unopened channel re-review rejection unavailable",async()=>{
 const id="4841d296-ef14-4c16-8d25-a7667e534feb";
 const app={id,companyName:"原企业",registrationNumber:"registration",categories:["COMPANY_REGISTRATION"],regions:["China"],fileIds:[id],state:"REJECTED",version:"8",agreementVersion:"current",agreementAccepted:false,onboardingState:"NOT_STARTED",reviewReason:"reviewed",updatedAt:"2026-10-08T00:00:00Z"};
 const show=(onboardingState:string)=>{vi.mocked(ecoRequest).mockResolvedValue({applications:[{...app,onboardingState}],total:"1"});return render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><EcoservicesJoin/></QueryClientProvider>)};
 show("NOT_STARTED");expect(await screen.findByRole("button",{name:"修正资料并重新提交 →"})).toBeEnabled();cleanup();
 show("PREPARING");await screen.findByText("原入驻申请");expect(screen.getByRole("button",{name:/修正资料并重新提交|提交机构入驻申请/})).toBeDisabled();
});
