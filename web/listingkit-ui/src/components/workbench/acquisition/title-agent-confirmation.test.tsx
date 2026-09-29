import { afterEach,beforeEach,expect,it,vi } from "vitest";
import { cleanup,fireEvent,render,screen,waitFor } from "@testing-library/react";
import { TitleAgentConfirmation } from "./title-agent-confirmation";
import { KnowledgeError } from "@/lib/api/knowledge";
const fixture=vi.hoisted(()=>({request:vi.fn()}));
vi.mock("@/lib/api/knowledge",async original=>({...await original<typeof import("@/lib/api/knowledge")>(),knowledgeRequest:fixture.request}));
const base="11111111-1111-4111-8111-111111111111",revision="22222222-2222-4222-8222-222222222222";
const props={scope:{userId:"actor",organizationId:"B"},organization:"企业 B",platform:"shein",knowledgeAvailable:true,onConfirm:vi.fn(),onCancel:vi.fn()};
beforeEach(()=>{vi.clearAllMocks();Object.defineProperty(HTMLDialogElement.prototype,"showModal",{configurable:true,value:function(this:HTMLDialogElement){this.open=true}});Object.defineProperty(HTMLDialogElement.prototype,"close",{configurable:true,value:function(this:HTMLDialogElement){this.open=false}})});
afterEach(cleanup);
it("loads selected current readable versions and passes only a base ID after explicit confirmation",async()=>{
 fixture.request.mockResolvedValueOnce({items:[{id:base,name:"品牌规范",state:"ACTIVE"}]}).mockResolvedValueOnce({items:[{id:base,name:"表达规范",state:"ACTIVE",currentReadableRevision:{id:revision,number:3,state:"PARTIAL"}}]});
 render(<TitleAgentConfirmation {...props}/>);
 expect(fixture.request).not.toHaveBeenCalled();expect(props.onConfirm).not.toHaveBeenCalled();
 fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));
 expect(screen.getByText("确认生成")).toBeDisabled();
 await screen.findByText("品牌规范");fireEvent.change(screen.getByLabelText("企业知识库"),{target:{value:base}});
 await screen.findByText("表达规范 · v3 · 部分解析可用");
 expect(fixture.request.mock.calls[1].slice(0,2)).toEqual([props.scope,`knowledge-bases/${base}/sources`]);
 fireEvent.click(screen.getByText("确认生成"));expect(props.onConfirm).toHaveBeenCalledExactlyOnceWith(base);
});
it("blocks unreadable/denied Knowledge and requires explicit opt-out before ordinary generation",async()=>{
 fixture.request.mockRejectedValueOnce(new KnowledgeError("KNOWLEDGE_FORBIDDEN",403));render(<TitleAgentConfirmation {...props}/>);
 fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));await screen.findByRole("alert");expect(screen.getByText("确认生成")).toBeDisabled();
 fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));fireEvent.click(screen.getByText("确认生成"));expect(props.onConfirm).toHaveBeenCalledExactlyOnceWith(undefined);
});
it("cancel aborts pending protected reads and never generates",async()=>{
 let resolve!:(v:unknown)=>void;fixture.request.mockImplementation(()=>new Promise(r=>{resolve=r}));const view=render(<TitleAgentConfirmation {...props}/>);
 fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));const signal=fixture.request.mock.calls[0][3].signal as AbortSignal;fireEvent.click(screen.getByText("取消"));expect(props.onCancel).toHaveBeenCalledOnce();view.unmount();expect(signal.aborted).toBe(true);
 resolve({items:[{id:base,name:"旧企业资料",state:"ACTIVE"}]});await waitFor(()=>expect(props.onConfirm).not.toHaveBeenCalled());
});
