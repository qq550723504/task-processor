import { KnowledgeOverview } from "@/components/workbench/knowledge/knowledge-overview";
import { ConsoleState } from "@/components/workbench/console/console-page";
import { isKnowledgeAvailable } from "@/lib/server/knowledge-availability";
export default function Page(){return isKnowledgeAvailable()?<KnowledgeOverview />:<ConsoleState kind="unavailable" title="知识库尚未开放" />;}
