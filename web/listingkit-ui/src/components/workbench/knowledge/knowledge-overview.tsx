import Link from "next/link";
import {ConsolePage} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import "./knowledge.css";

export function KnowledgeOverview() {
  return <ConsolePage title="知识库" description="知识库完全可选。企业共享资料默认不会自动引用。" breadcrumbs={[{label:"AI工作台",href:"/workbench/ai"},{label:"知识库"}]}>
    <Card className="knowledge-overview-notice">只有本次任务或模板明确选择后，才会读取获准的知识资料。</Card>
    <div className="knowledge-entry-grid">
      <Card className="knowledge-entry-card" data-kind="official">
        <h2>官方知识库</h2>
        <p className="console-description">由硕米维护和更新的知识内容入口，按需选择。</p>
        <div className="knowledge-entry-actions"><span className="knowledge-status">官方知识内容尚未开放</span><Button asChild variant="outline"><Link href="/workbench/ai/knowledge/official" prefetch={false}>查看官方知识库 →</Link></Button></div>
      </Card>
      <Card className="knowledge-entry-card" data-kind="mine">
        <h2>我的知识库</h2>
        <p className="console-description">管理当前企业共享资料，按需创建知识库并上传资料。</p>
        <div className="knowledge-entry-actions"><span className="knowledge-status">当前企业资料</span><Button asChild variant="outline"><Link href="/workbench/ai/knowledge/mine" prefetch={false}>查看我的知识库 →</Link></Button></div>
      </Card>
    </div>
  </ConsolePage>;
}
