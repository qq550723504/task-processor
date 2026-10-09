import {cleanup,render,screen,waitFor} from "@testing-library/react";
import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {afterEach,expect,it,vi} from "vitest";
import {ecoRequest} from "@/lib/api/ecoservices";
import {FinancialFacts} from "./financial";
import {useEcoCommands} from "./shared";

const context=vi.hoisted(()=>({user:{id:"actor"},effectiveOrganization:{id:"org"},permissions:["platform.ecoservices.review"],isLoading:false,isSwitching:false,error:null,blockingError:null,registerOrganizationSwitchGuard:vi.fn(()=>()=>undefined)}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>context}));
vi.mock("@/lib/api/ecoservices",async original=>({...await original<typeof import("@/lib/api/ecoservices")>(),ecoRequest:vi.fn()}));
afterEach(()=>{cleanup();vi.clearAllMocks()});
const scope={userId:"actor",organizationId:"org"},id="4841d296-ef14-4c16-8d25-a7667e534feb";
function Panel(){const commands=useEcoCommands(scope);return <FinancialFacts scope={scope} id={id} commands={commands}/>}

it.each([["0","¥0.00"],["20","¥0.20"],["101","¥1.01"]])("shows canonical chargeback %s separately from confirmed refunds",async(chargedBackMinor,amount)=>{
 const client=new QueryClient({defaultOptions:{queries:{retry:false}}});
 vi.mocked(ecoRequest).mockResolvedValue({grossMinor:"101",refundedMinor:"2",chargedBackMinor,platformMinor:"7",providerMinor:"72",sharedMinor:"10",returnedMinor:"3",releasedMinor:"91",channelFeeMinor:"0",channelFeeObserved:false,reconciliationReason:""});
 render(<QueryClientProvider client={client}><Panel/></QueryClientProvider>);
 await waitFor(()=>expect(screen.queryByText("正在读取原资金记录…")).not.toBeInTheDocument());
 const term=screen.getByText("已确认拒付",{selector:"dt"});expect(term.nextElementSibling?.textContent).toBe(amount);
 const refunds=screen.getByText("已确认退款",{selector:"dt"});expect(refunds.nextElementSibling?.textContent).toBe("¥0.02");
 expect(screen.getByText("尚无账单事实")).toBeVisible();
 expect(vi.mocked(ecoRequest).mock.calls.every(v=>v[4]===true&&!v[3]?.method)).toBe(true);client.clear();
});
