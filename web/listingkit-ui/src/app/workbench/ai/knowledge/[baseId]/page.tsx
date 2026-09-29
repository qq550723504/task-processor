import { KnowledgePage } from "@/components/workbench/knowledge/knowledge-page";
import { knowledgeId } from "@/lib/api/knowledge";
import { notFound } from "next/navigation";
import { ConsoleState } from "@/components/workbench/console/console-page";
import { isKnowledgeAvailable } from "@/lib/server/knowledge-availability";
export default async function Page({params}:{params:Promise<{baseId:string}>}){const {baseId}=await params;if(!knowledgeId.safeParse(baseId).success)notFound();return isKnowledgeAvailable()?<KnowledgePage baseId={baseId} />:<ConsoleState kind="unavailable" title="知识库尚未开放" />;}
