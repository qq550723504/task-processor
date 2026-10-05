import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import {useEffect} from "react";
import { ProductAgentPanel } from "./product-agent-panel";
import { ProductAgentError } from "@/lib/api/product-agent";
const fixture = vi.hoisted(() => ({ request: vi.fn(), knowledge:vi.fn(), params: new URLSearchParams(), context: { user: { id: "actor" }, effectiveOrganization: { id: "org" }, roles:["listingkit_operator"], isLoading: false, isSwitching: false, error: null, blockingError: null, selectionRequired: false, registerOrganizationSwitchGuard: vi.fn(() => () => { }) } }));
vi.mock("@/lib/api/knowledge",async original=>({...await original<typeof import("@/lib/api/knowledge")>(),knowledgeRequest:fixture.knowledge}));
vi.mock("@/lib/api/product-agent", async (importOriginal) => ({ ...await importOriginal<typeof import("@/lib/api/product-agent")>(), requestProductAgent: fixture.request }));
vi.mock("next/navigation", () => ({ useSearchParams: () => fixture.params }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));
vi.mock("./title-agent-templates",()=>({TitleAgentTemplates:({onReady}:{onReady:(v:boolean)=>void})=>{useEffect(()=>onReady(true),[onReady]);return null}}));
beforeEach(() => { fixture.context.roles=["listingkit_operator"];fixture.request.mockReset();fixture.knowledge.mockReset(); fixture.params = new URLSearchParams(); window.history.replaceState(null, "", "/"); Object.defineProperty(HTMLDialogElement.prototype,"showModal",{configurable:true,value:function(this:HTMLDialogElement){this.open=true}});Object.defineProperty(HTMLDialogElement.prototype,"close",{configurable:true,value:function(this:HTMLDialogElement){this.open=false}}); });
afterEach(cleanup);
const op = "11111111-1111-4111-8111-111111111111";
const props = { enabled: true, operationId: op, productKey: "product", catalogVersion: "1" };
it("keeps a closed capability explicit and never dispatches on mount", () => {
    render(<ProductAgentPanel {...props} enabled={false}/>);
    expect(screen.getByText("当前环境尚未开放 Product Agent。")).toBeInTheDocument();
    expect(fixture.request).not.toHaveBeenCalled();
});
it("retains one request key after lost response without automatically retrying", async () => {
    fixture.request.mockRejectedValue(new ProductAgentError("OUTCOME_UNKNOWN"));
    render(<ProductAgentPanel {...props}/>);
    expect(screen.getByText("生成标题建议")).toBeDisabled();
    fireEvent.change(screen.getByLabelText("素材查询平台"), { target: { value: "shein" } });
    fireEvent.click(screen.getByText("生成标题建议"));
    expect(fixture.request).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog",{name:"确认标题优化"})).toBeInTheDocument();
    fireEvent.click(screen.getByText("确认生成"));
    await screen.findByRole("alert");
    expect(fixture.request).toHaveBeenCalledTimes(1);
    expect(fixture.request.mock.calls[0][7]).toBe("shein");
    const key = fixture.request.mock.calls[0][2];
    expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(key);
    fireEvent.click(screen.getByText("读取当前结果"));
    await waitFor(() => expect(fixture.request).toHaveBeenCalledTimes(2));
    expect(fixture.request.mock.calls[1].slice(0, 3)).toEqual(["read", op, key]);
    expect(screen.queryByText("生成标题建议")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name:"重新确认配置"})).toBeNull();
});

it.each(["CONFIGURATION_CHANGED", "TEMPLATE_ARCHIVED", "AGENT_NOT_ENABLED", "AGENT_DEFINITION_UNAVAILABLE"])("offers explicit new confirmation only after a zero-Claim Start rejection: %s", async code => {
    fixture.request.mockRejectedValueOnce(new ProductAgentError(code)).mockRejectedValueOnce(new ProductAgentError("OUTCOME_UNKNOWN"));
    render(<ProductAgentPanel {...props}/>);
    fireEvent.change(screen.getByLabelText("素材查询平台"), { target: { value: "shein" } });
    fireEvent.click(screen.getByText("生成标题建议"));
    fireEvent.click(screen.getByText("确认生成"));
    await screen.findByRole("alert");
    const firstKey = fixture.request.mock.calls[0][2];
    expect(fixture.request).toHaveBeenCalledTimes(1);
    expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(firstKey);
    fireEvent.click(screen.getByRole("button", {name:"重新确认配置"}));
    expect(new URL(window.location.href).searchParams.has("agent_key")).toBe(false);
    expect(fixture.request).toHaveBeenCalledTimes(1);
    fireEvent.change(screen.getByLabelText("素材查询平台"), { target: { value: "shein" } });
    fireEvent.click(screen.getByText("生成标题建议"));
    fireEvent.click(screen.getByText("确认生成"));
    await waitFor(() => expect(fixture.request).toHaveBeenCalledTimes(2));
    expect(fixture.request.mock.calls[1][2]).not.toBe(firstKey);
    await screen.findByRole("alert");
    expect(screen.queryByRole("button", {name:"重新确认配置"})).toBeNull();
});

it.each(["CONFIGURATION_CHANGED", "AGENT_DEFINITION_UNAVAILABLE"])("preserves an interrupted run after %s on Resume", async code => {
    fixture.request.mockResolvedValueOnce({runId:op,requestKey:op,operationId:op,productKey:"product",catalogVersion:"1",targetPlatform:"shein",phase:"interrupted",revision:"2",canSubmitReview:false,candidate:{Changes:[]},confidence:[],unresolved:[],steps:[],tokens:0,estimatedCostMicros:0,currency:"CNY",usageStatus:"observed"}).mockRejectedValueOnce(new ProductAgentError(code));
    render(<ProductAgentPanel {...props}/>);
    fireEvent.change(screen.getByLabelText("素材查询平台"), { target: { value: "shein" } });
    fireEvent.click(screen.getByText("生成标题建议"));fireEvent.click(screen.getByText("确认生成"));
    await screen.findByText("状态：需要补充说明");
    const firstKey = fixture.request.mock.calls[0][2];
    fireEvent.change(screen.getByLabelText("补充说明"), {target:{value:"继续"}});
    fireEvent.click(screen.getByText("继续本次诊断"));
    await screen.findByRole("alert");
    expect(fixture.request.mock.calls[1][0]).toBe("resume");
    expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(firstKey);
    expect(screen.queryByRole("button", {name:"重新确认配置"})).toBeNull();
});

it.each(["CONFIGURATION_CHANGED", "AGENT_DEFINITION_UNAVAILABLE"])("allows reconfirmation after an original restored Start receives a definite zero-Claim response: %s", async code => {
    fixture.params = new URLSearchParams({agent_key:op,agent_platform:"shein",agent_actor:"actor",agent_org:"org"});
    window.history.replaceState(null,"",`/?${fixture.params}`);
    fixture.request.mockRejectedValueOnce(new ProductAgentError(code));
    render(<ProductAgentPanel {...props}/>);
    fireEvent.click(screen.getByText("使用原请求核实启动"));
    await screen.findByRole("alert");
    expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(op);
    fireEvent.click(screen.getByRole("button", {name:"重新确认配置"}));
    expect(new URL(window.location.href).searchParams.has("agent_key")).toBe(false);
    expect(fixture.request).toHaveBeenCalledTimes(1);
    expect(screen.getByText("生成标题建议")).toBeInTheDocument();
});
it("only admits a validated candidate to existing human review", async () => {
    fixture.request.mockResolvedValue({ runId: op, requestKey: op, operationId: op, productKey: "product", catalogVersion: "1", targetPlatform: "shein", phase: "human_review_required", revision: "2", canSubmitReview: true, candidate: { Changes: [{ Field: "title", Value: "建议标题", EvidenceIDs: ["evidence"] }] }, confidence: [], unresolved: [], steps: [], tokens: 30, estimatedCostMicros: 20, currency: "CNY", usageStatus: "observed" });
    render(<ProductAgentPanel {...props}/>);
    expect(screen.getByText("生成标题建议")).toBeDisabled();
    fireEvent.change(screen.getByLabelText("素材查询平台"), { target: { value: "shein" } });
    fireEvent.click(screen.getByText("生成标题建议"));
    fireEvent.click(screen.getByText("确认生成"));
    await screen.findByText("title：建议标题");
    expect(screen.getByText("状态：候选已通过校验，等待人工审核")).toBeInTheDocument();
    expect(screen.queryByText("自动应用")).not.toBeInTheDocument();
    fixture.request.mockResolvedValue({ proposalId: op });
    fireEvent.click(screen.getByText("提交人工审核"));
    expect(await screen.findByRole("link", { name: "打开标题审核" })).toHaveAttribute("href", `/workbench/ai/tasks/pending/other?proposal_id=${op}`);
    expect(fixture.request.mock.calls[1][0]).toBe("review");
});

it("shows an invalid repair-limit candidate without claiming validation or offering review", async () => {
    fixture.request.mockResolvedValue({ runId: op, requestKey: op, operationId: op, productKey: "product", catalogVersion: "1", targetPlatform: "shein", phase: "human_review_required", revision: "2", stopReason: "repair_limit", canSubmitReview: false, candidate: { Changes: [{ Field: "title", Value: "无效标题", EvidenceIDs: ["evidence"] }] }, confidence: [], unresolved: ["标题证据不足"], steps: [], tokens: 30, estimatedCostMicros: 20, currency: "CNY", usageStatus: "observed" });
    render(<ProductAgentPanel {...props}/>);
    fireEvent.change(screen.getByLabelText("素材查询平台"), { target: { value: "shein" } });
    fireEvent.click(screen.getByText("生成标题建议"));
    fireEvent.click(screen.getByText("确认生成"));
    await screen.findByText("title：无效标题");
    expect(screen.getByText("状态：候选未通过校验，不能提交人工审核")).toBeInTheDocument();
    expect(screen.getByText("两次修复后仍未通过校验")).toBeInTheDocument();
    expect(screen.queryByText("状态：候选已通过校验，等待人工审核")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "提交人工审核" })).not.toBeInTheDocument();
    expect(fixture.request).toHaveBeenCalledTimes(1);
});
it("restores the original normalized Start after refresh and explicitly retries the same key and selection",async()=>{
 fixture.request.mockRejectedValueOnce(new ProductAgentError("DEPENDENCY_UNAVAILABLE"));
 fixture.knowledge.mockResolvedValueOnce({items:[{id:op,name:"Private guide",state:"ACTIVE"}]}).mockResolvedValueOnce({items:[{id:op,name:"Private source",state:"ACTIVE",currentReadableRevision:{id:op,number:1,state:"AVAILABLE"}}]});
 const view=render(<ProductAgentPanel {...props} knowledgeAvailable/>);
 fireEvent.change(screen.getByLabelText("素材查询平台"),{target:{value:"shein"}});fireEvent.click(screen.getByText("生成标题建议"));fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));await screen.findByText("Private guide");fireEvent.change(screen.getByLabelText("企业知识库"),{target:{value:op}});await screen.findByText("Private source · v1 · 可读");fireEvent.click(screen.getByText("确认生成"));await screen.findByRole("alert");
 const first=fixture.request.mock.calls[0];expect(first[8]).toBe(op);
 expect(window.location.href).not.toContain("Private");view.unmount();fixture.params=new URL(window.location.href).searchParams;
 render(<ProductAgentPanel {...props} knowledgeAvailable/>);expect(fixture.request).toHaveBeenCalledTimes(1);expect(screen.queryByText("Private guide")).toBeNull();
 fixture.request.mockRejectedValueOnce(new ProductAgentError("KNOWLEDGE_DISABLED"));fireEvent.click(screen.getByText("使用原请求核实启动"));await waitFor(()=>expect(fixture.request).toHaveBeenCalledTimes(2));await screen.findByRole("alert");
 expect(fixture.request.mock.calls[1].slice(0,4)).toEqual(first.slice(0,4));expect(fixture.request.mock.calls[1][7]).toBe("shein");expect(fixture.request.mock.calls[1][8]).toBe(op);expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(first[2]);expect(screen.queryByText("生成标题建议")).toBeNull();expect(screen.getByRole("alert")).not.toHaveTextContent("本次未启动");
});
it.each(["other actor","other org","invalid selection"])("does not restore Start intent from another scope or malformed URL: %s",variant=>{
 fixture.params=new URLSearchParams({agent_key:op,agent_platform:"shein",agent_actor:variant==="other actor"?"other":"actor",agent_org:variant==="other org"?"other":"org",agent_knowledge_base:variant==="invalid selection"?"invalid":op});render(<ProductAgentPanel {...props}/>);expect(screen.queryByText("使用原请求核实启动")).toBeNull();expect(screen.getByText("读取当前结果")).toBeInTheDocument();expect(fixture.request).not.toHaveBeenCalled();
});
it.each([false,true])("hides protected results after same-org role downgrade including late response=%s without losing key",async(late)=>{
 const response={runId:op,requestKey:op,operationId:op,productKey:"product",catalogVersion:"1",targetPlatform:"shein",phase:"human_review_required",revision:"2",canSubmitReview:true,candidate:{Changes:[]},confidence:[],unresolved:[],steps:[],tokens:30,estimatedCostMicros:20,currency:"CNY",usageStatus:"observed",knowledge:{status:"available",originAgentRunId:op,citations:[{id:op,sourceId:op,revisionId:op,name:"Protected company guide",location:"text",excerpt:"Private wording",state:"AVAILABLE"}]}};
 let resolve!:(value:typeof response)=>void;fixture.request.mockImplementation(()=>new Promise(r=>{resolve=r}));
 const view=render(<ProductAgentPanel {...props}/>);fireEvent.change(screen.getByLabelText("素材查询平台"),{target:{value:"shein"}});fireEvent.click(screen.getByText("生成标题建议"));fireEvent.click(screen.getByText("确认生成"));
 const key=fixture.request.mock.calls[0][2];
 if(!late){resolve(response);await screen.findByText("Protected company guide · 可读")}
 fixture.context.roles=["listingkit_viewer"];view.rerender(<ProductAgentPanel {...props}/>);
 if(late)resolve(response);
 await screen.findByText(/知识当前不可用/);expect(screen.queryByText("Protected company guide · 可读")).toBeNull();expect(screen.queryByText("Private wording")).toBeNull();expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(key);
 fixture.context.roles=["listingkit_operator"];view.rerender(<ProductAgentPanel {...props}/>);expect(screen.queryByText("Protected company guide · 可读")).toBeNull();expect(fixture.request).toHaveBeenCalledTimes(1);
});
it("cannot revive an old protected response after downgrade and regrant before completion",async()=>{
 const response={runId:op,requestKey:op,operationId:op,productKey:"product",catalogVersion:"1",targetPlatform:"shein",phase:"human_review_required",revision:"2",canSubmitReview:true,candidate:{Changes:[]},confidence:[],unresolved:[],steps:[],tokens:30,estimatedCostMicros:20,currency:"CNY",usageStatus:"observed",knowledge:{status:"available",originAgentRunId:op,citations:[{id:op,sourceId:op,revisionId:op,name:"Old protected guide",location:"text",excerpt:"Old protected excerpt",state:"AVAILABLE"}]}};
 let resolve!:(value:typeof response)=>void;fixture.request.mockImplementation(()=>new Promise(r=>{resolve=r}));const view=render(<ProductAgentPanel {...props}/>);fireEvent.change(screen.getByLabelText("素材查询平台"),{target:{value:"shein"}});fireEvent.click(screen.getByText("生成标题建议"));fireEvent.click(screen.getByText("确认生成"));const key=fixture.request.mock.calls[0][2];
 fixture.context.roles=["listingkit_viewer"];view.rerender(<ProductAgentPanel {...props}/>);fixture.context.roles=["listingkit_operator"];view.rerender(<ProductAgentPanel {...props}/>);resolve(response);
 await screen.findByText(/知识当前不可用/);expect(screen.queryByText("Old protected guide · 可读")).toBeNull();expect(screen.queryByText("Old protected excerpt")).toBeNull();expect(new URL(window.location.href).searchParams.get("agent_key")).toBe(key);expect(fixture.request).toHaveBeenCalledTimes(1);
});
