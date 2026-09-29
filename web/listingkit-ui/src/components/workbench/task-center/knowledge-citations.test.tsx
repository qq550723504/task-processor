import {afterEach,expect,it} from "vitest";
import {cleanup,render,screen} from "@testing-library/react";
import {KnowledgeCitations} from "./knowledge-citations";
import {productKnowledgeSchema} from "@/lib/contracts/product-knowledge";
const id="11111111-1111-4111-8111-111111111111";
const citation={id,sourceId:id,revisionId:id,name:"品牌表达",location:"text",excerpt:"<script>untrusted</script>",state:"PARTIAL" as const};
afterEach(cleanup);
it("separates original AI support from edited title and renders protected text as plain text",()=>{
 render(<KnowledgeCitations knowledge={{status:"available",originAgentRunId:id,citations:[citation]}} humanEdited/>);
 expect(screen.getByText("以下出处属于原 AI 建议，未重新证明人工编辑后的标题。")).toBeInTheDocument();expect(screen.getByText("品牌表达 · 部分解析可用")).toBeInTheDocument();expect(document.querySelector("script")).toBeNull();
});
it("unavailable has no protected projection and unannotated output claims no citation",()=>{
 expect(productKnowledgeSchema.safeParse({status:"unavailable",originAgentRunId:id,citations:[citation]}).success).toBe(false);
 const view=render(<KnowledgeCitations knowledge={{status:"unavailable",originAgentRunId:id,citations:[]}}/>);expect(screen.getByText(/知识当前不可用/)).toBeInTheDocument();expect(screen.queryByText("品牌表达")).toBeNull();
 view.rerender(<KnowledgeCitations knowledge={{status:"uncited",originAgentRunId:id,citations:[]}}/>);expect(screen.getByText("本次采用企业知识上下文，模型未标注具体出处。")).toBeInTheDocument();
});
