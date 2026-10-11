import {notFound} from "next/navigation";
import {OfficialKnowledgePage} from "@/components/workbench/knowledge/official-knowledge-page";
import {officialArticleID, officialRevision} from "@/lib/api/official-knowledge";

export default async function Page({params}: {params: Promise<{articleId: string; revision: string}>}) {
  const {articleId, revision} = await params;
  if (!officialArticleID.safeParse(articleId).success || !officialRevision.safeParse(revision).success) notFound();
  return <OfficialKnowledgePage articleId={articleId} revision={revision}/>;
}
