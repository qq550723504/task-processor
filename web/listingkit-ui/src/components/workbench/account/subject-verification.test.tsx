import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { SubjectVerification } from "./subject-verification";
const clients:QueryClient[]=[];
function mount(){const client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});clients.push(client);return render(<QueryClientProvider client={client}><SubjectVerification userId="user" organizationId="org" /></QueryClientProvider>);}
afterEach(()=>{cleanup();clients.splice(0).forEach(c=>c.clear());vi.unstubAllGlobals();});
const initial={state:"NOT_STARTED",userId:"user",organizationId:"org",canStart:true,maskedPhone:"138****0001"};
it("submits once and keeps uncertain outcomes blocked while allowing status refresh",async()=>{
 const fetch=vi.fn().mockImplementation((_url:string,init?:RequestInit)=>Promise.resolve(init?.method==="POST"?Response.json({code:"RESULT_UNVERIFIED"},{status:502}):Response.json(initial)));vi.stubGlobal("fetch",fetch);mount();await userEvent.click(screen.getByRole("button",{name:"企业认证"}));
 const user=userEvent.setup();await screen.findByLabelText("企业名称");await user.type(screen.getByLabelText("企业名称"),"测试企业");await user.type(screen.getByLabelText("统一社会信用代码"),"91310000MA00000001");
 await user.click(screen.getByRole("checkbox"));await user.click(screen.getByRole("button",{name:"前往腾讯电子签认证"}));
 expect(await screen.findByText(/申请结果待核实/)).toBeVisible();expect(screen.queryByRole("link",{name:"继续认证"})).not.toBeInTheDocument();
 await user.click(screen.getByRole("button",{name:"刷新认证状态"}));expect(fetch.mock.calls.filter(c=>c[1]?.method==="POST")).toHaveLength(1);expect(screen.queryByRole("button",{name:"前往腾讯电子签认证"})).not.toBeInTheDocument();
});
it("shows verified enterprise facts without elevating personal or platform authorization",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockImplementation(()=>Promise.resolve(Response.json({...initial,state:"VERIFIED",canStart:false,applicationId:"app",companyName:"测试企业",creditCode:"91310000MA00000001",verifiedAt:"2026-09-26T00:00:00Z",isApplicant:true}))));mount();await userEvent.click(screen.getByRole("button",{name:"企业认证"}));
 expect(await screen.findByText("企业认证已通过")).toBeVisible();expect(screen.getByText(/认证不会增加平台权限/)).toBeVisible();
 await userEvent.click(screen.getByRole("button",{name:"个人认证"}));expect(await screen.findByText("个人认证暂不可用")).toBeVisible();
});
