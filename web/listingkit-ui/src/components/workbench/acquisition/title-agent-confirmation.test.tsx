import { afterEach,beforeEach,expect,it,vi } from "vitest";
import { cleanup,fireEvent,render,screen,waitFor } from "@testing-library/react";
import { TitleAgentConfirmation } from "./title-agent-confirmation";
import { KnowledgeError } from "@/lib/api/knowledge";
const fixture=vi.hoisted(()=>({request:vi.fn()}));
vi.mock("@/lib/api/knowledge",async original=>({...await original<typeof import("@/lib/api/knowledge")>(),knowledgeRequest:fixture.request}));
const base="11111111-1111-4111-8111-111111111111",revision="22222222-2222-4222-8222-222222222222";
const props={scope:{userId:"actor",organizationId:"B"},organization:"企业 B",platform:"shein",knowledgeAvailable:true,onConfirm:vi.fn(),onCancel:vi.fn()};
beforeEach(()=>{vi.clearAllMocks();fixture.request.mockReset();Object.defineProperty(HTMLDialogElement.prototype,"showModal",{configurable:true,value:function(this:HTMLDialogElement){this.open=true}});Object.defineProperty(HTMLDialogElement.prototype,"close",{configurable:true,value:function(this:HTMLDialogElement){this.open=false}})});
afterEach(cleanup);
it("loads selected current readable versions and passes only a base ID after explicit confirmation",async()=>{
 fixture.request.mockResolvedValueOnce({items:[{id:base,name:"品牌规范",state:"ACTIVE"}],pagination:{page:1,pageSize:100,total:1}}).mockResolvedValueOnce({items:[{id:base,name:"表达规范",state:"ACTIVE",currentReadableRevision:{id:revision,number:3,state:"PARTIAL"}}]});
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
 resolve({items:[{id:base,name:"旧企业资料",state:"ACTIVE"}],pagination:{page:1,pageSize:100,total:1}});await waitFor(()=>expect(props.onConfirm).not.toHaveBeenCalled());
});
it("removes earlier protected base names when the following source read loses permission",async()=>{
 fixture.request.mockResolvedValueOnce({items:[{id:base,name:"Private base name",state:"ACTIVE"}],pagination:{page:1,pageSize:100,total:1}}).mockRejectedValueOnce(new KnowledgeError("KNOWLEDGE_FORBIDDEN",403));render(<TitleAgentConfirmation {...props}/>);
 fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));await screen.findByText("Private base name");fireEvent.change(screen.getByLabelText("企业知识库"),{target:{value:base}});await screen.findByRole("alert");expect(screen.queryByText("Private base name")).toBeNull();expect(screen.getByText("确认生成")).toBeDisabled();expect(props.onConfirm).not.toHaveBeenCalled();
});
it("reaches later Knowledge pages and clears the previous selection before generation",async()=>{
 const later="33333333-3333-4333-8333-333333333333";
 const first=Array.from({length:100},(_,i)=>({id:i===0?base:`10000000-0000-4000-8000-${String(i).padStart(12,"0")}`,name:`知识库 ${i+1}`,state:"ACTIVE"}));
 const sources={items:[{id:base,name:"表达规范",state:"ACTIVE",currentReadableRevision:{id:revision,number:3,state:"AVAILABLE"}}]};
 fixture.request.mockResolvedValueOnce({items:first,pagination:{page:1,pageSize:100,total:101}}).mockResolvedValueOnce(sources)
  .mockResolvedValueOnce({items:[{id:later,name:"第 101 个知识库",state:"ACTIVE"}],pagination:{page:2,pageSize:100,total:101}}).mockResolvedValueOnce(sources);
 render(<TitleAgentConfirmation {...props}/>);fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));
 await screen.findByText("知识库 1");expect(fixture.request).toHaveBeenCalledTimes(1);
 fireEvent.change(screen.getByLabelText("企业知识库"),{target:{value:base}});await waitFor(()=>expect(screen.getByText("确认生成")).toBeEnabled());
 fireEvent.click(screen.getByText("下一页"));expect(screen.getByText("确认生成")).toBeDisabled();expect(screen.getByLabelText("企业知识库")).toHaveValue("");
 await screen.findByText("第 101 个知识库");expect(fixture.request.mock.calls[2].slice(0,2)).toEqual([props.scope,"knowledge-bases?page=2&pageSize=100"]);
 expect(screen.queryByText("知识库 1")).toBeNull();expect(screen.getByText("下一页")).toBeDisabled();expect(screen.getByText("上一页")).toBeEnabled();
 expect(props.onConfirm).not.toHaveBeenCalled();fireEvent.change(screen.getByLabelText("企业知识库"),{target:{value:later}});
 await waitFor(()=>expect(screen.getByText("确认生成")).toBeEnabled());fireEvent.click(screen.getByText("确认生成"));expect(props.onConfirm).toHaveBeenCalledExactlyOnceWith(later);
});
it("clears protected names and selection when a later Knowledge page loses permission",async()=>{
 fixture.request.mockResolvedValueOnce({items:[{id:base,name:"Private page name",state:"ACTIVE"}],pagination:{page:1,pageSize:100,total:101}})
  .mockRejectedValueOnce(new KnowledgeError("KNOWLEDGE_FORBIDDEN",403));
 render(<TitleAgentConfirmation {...props}/>);fireEvent.click(screen.getByLabelText("使用企业知识（可选）"));await screen.findByText("Private page name");
 fireEvent.click(screen.getByText("下一页"));await screen.findByRole("alert");expect(screen.queryByText("Private page name")).toBeNull();
 expect(screen.getByLabelText("企业知识库")).toHaveValue("");expect(screen.getByText("确认生成")).toBeDisabled();expect(props.onConfirm).not.toHaveBeenCalled();
});
