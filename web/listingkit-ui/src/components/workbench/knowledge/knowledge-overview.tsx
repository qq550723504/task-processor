import Link from "next/link";
import {ConsolePage} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import "./knowledge.css";

export function KnowledgeOverview({enterpriseAvailable = false}: {enterpriseAvailable?: boolean}) {
  return <ConsolePage title="知识库" description="知识库完全可选。企业共享资料默认不会自动引用。" breadcrumbs={[{label:"AI工作台",href:"/workbench/ai"},{label:"知识库"}]}>
    <Card className="knowledge-overview-notice">只有本次任务或模板明确选择后，才会读取获准的知识资料。</Card>
    <div className="knowledge-entry-grid">
      <Card className="knowledge-entry-card" data-kind="official">
        <h2>官方知识库</h2>
        <p className="console-description">由硕米维护的应用指南，查看正文、版本和出处。AI 引用尚未开放。</p>
        <div className="knowledge-entry-actions"><span className="knowledge-status">硕米维护 · 只读资料</span><Button asChild variant="outline"><Link href="/workbench/ai/knowledge/official" prefetch={false}>查看官方知识库 →</Link></Button></div>
      </Card>
      <Card className="knowledge-entry-card" data-kind="mine">
        <h2>我的知识库</h2>
        <p className="console-description">管理当前企业共享资料，按需创建知识库并上传资料。</p>
        <div className="knowledge-entry-actions"><span className="knowledge-status">{enterpriseAvailable ? "当前企业资料" : "企业知识库尚未开放"}</span>{enterpriseAvailable ? <Button asChild variant="outline"><Link href="/workbench/ai/knowledge/mine" prefetch={false}>查看我的知识库 →</Link></Button> : null}</div>
      </Card>
    </div>
  </ConsolePage>;
}
