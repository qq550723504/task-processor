import {render,screen} from "@testing-library/react";
import {expect,it,vi} from "vitest";
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>({user:null,isLoading:true,error:{code:"DEPENDENCY_UNAVAILABLE"},blockingError:null,effectiveOrganization:null})}));
import {MarketBoundary} from "./shared";
it("uses the specialist server identity independently of enterprise availability",()=>{
 render(<MarketBoundary admin expectedUserId="verified-specialist">{scope=><p>{scope.userId}:{scope.organizationId||"platform"}</p>}</MarketBoundary>);
 expect(screen.getByText("verified-specialist:platform")).toBeVisible();
});
it("keeps member market access behind enterprise confirmation",()=>{
 render(<MarketBoundary>{()=> <p>hidden member</p>}</MarketBoundary>);
 expect(screen.queryByText("hidden member")).not.toBeInTheDocument();
});
