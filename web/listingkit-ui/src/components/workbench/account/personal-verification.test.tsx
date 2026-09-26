import { QueryClient,QueryClientProvider } from "@tanstack/react-query";
import { cleanup,render,screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach,expect,it,vi } from "vitest";
import { PersonalVerification } from "./personal-verification";
const initial={userId:"user",state:"NOT_STARTED",maskedPhone:"138****0001",canStart:true,canRefresh:false,phoneReady:true,quota:{totalLimit:5,totalUsed:0,totalRemaining:5,dailyLimit:3,dailyUsed:0,dailyRemaining:3,serverTime:"2026-09-27T01:00:00Z",resetAt:"2026-09-27T16:00:00Z",nextAllowedAt:"2026-09-27T01:00:00Z"}};
const clients:QueryClient[]=[];
function mount(){const c=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});clients.push(c);render(<QueryClientProvider client={c}><PersonalVerification userId="user"/></QueryClientProvider>);}
afterEach(()=>{cleanup();clients.splice(0).forEach(c=>c.clear());vi.unstubAllGlobals();});
it("shows lifetime cap even when daily quota reset",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(Response.json({...initial,state:"REJECTED",applicationId:"eab2c3a9-a112-4a15-a679-f5b3841a1003",canStart:false,quota:{...initial.quota,totalUsed:5,totalRemaining:0}})));mount();expect(await screen.findByText("累计剩余 0 / 5 次")).toBeVisible();expect(screen.getByText(/累计认证次数已用完/)).toBeVisible();expect(screen.queryByRole("button",{name:"开始个人认证"})).not.toBeInTheDocument();
});
it("does not spend verification attempts just by opening or refreshing the page",async()=>{
 const fetch=vi.fn().mockImplementation(()=>Promise.resolve(Response.json(initial)));vi.stubGlobal("fetch",fetch);mount();await screen.findByLabelText("真实姓名");await userEvent.click(screen.getByRole("button",{name:"刷新认证结果"}));expect(fetch.mock.calls.every(c=>c[1]?.method==="GET")).toBe(true);
});
it("submits once, clears identity text, and continues the same application at the cap",async()=>{
 const pending={...initial,state:"PENDING",applicationId:"eab2c3a9-a112-4a15-a679-f5b3841a1003",canStart:false,canRefresh:true,expiresAt:"2026-09-27T01:30:00Z",verificationUrl:"https://t.aliyun.com/example",quota:{...initial.quota,totalUsed:5,totalRemaining:0,dailyUsed:3,dailyRemaining:0}};
 let started=false;
 const fetch=vi.fn().mockImplementation((url:string,init?:RequestInit)=>{
  if(init?.method==="POST"&&url.endsWith("/applications"))started=true;
  return Promise.resolve(Response.json(started?pending:initial));
 });vi.stubGlobal("fetch",fetch);vi.stubGlobal("getMetaInfo",()=>({deviceType:"pc"}));mount();
 const user=userEvent.setup();await screen.findByLabelText("真实姓名");await user.type(screen.getByLabelText("真实姓名"),"测试姓名");await user.type(screen.getByLabelText("身份证号码"),"110101199001010010");await user.click(screen.getByRole("checkbox"));await user.click(screen.getByRole("button",{name:"开始个人认证"}));
 expect(await screen.findByRole("link",{name:"继续个人认证"})).toHaveAttribute("href",pending.verificationUrl);expect(screen.queryByDisplayValue("110101199001010010")).not.toBeInTheDocument();expect(screen.getByText("累计剩余 0 / 5 次")).toBeVisible();
 await user.click(screen.getByRole("button",{name:"刷新认证结果"}));expect(fetch.mock.calls.filter(c=>c[0].endsWith("/applications"))).toHaveLength(1);expect(fetch.mock.calls.filter(c=>c[0].endsWith("/refresh"))).toHaveLength(1);
});
