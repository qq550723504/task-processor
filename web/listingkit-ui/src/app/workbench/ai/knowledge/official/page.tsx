import {ConsolePage, ConsoleState} from "@/components/workbench/console/console-page";

export default function Page() {
  return <ConsolePage title="官方知识库" description="由硕米维护的知识内容入口。" breadcrumbs={[{label:"AI工作台",href:"/workbench/ai"},{label:"知识库",href:"/workbench/ai/knowledge"},{label:"官方知识库"}]}>
    <ConsoleState kind="unavailable" title="官方知识内容尚未开放">开放后会提供正式知识内容。当前可在“我的知识库”管理企业资料。</ConsoleState>
  </ConsolePage>;
}
