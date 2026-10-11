import { KnowledgeOverview } from "@/components/workbench/knowledge/knowledge-overview";
import { isKnowledgeAvailable } from "@/lib/server/knowledge-availability";
export default function Page(){return <KnowledgeOverview enterpriseAvailable={isKnowledgeAvailable()} />;}
