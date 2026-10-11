"use client";

import Link from "next/link";
import {useEffect, useMemo, useState} from "react";
import {useQuery, useQueryClient} from "@tanstack/react-query";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {ConsolePage, ConsoleState} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import {KnowledgeError, type KnowledgeScope} from "@/lib/api/knowledge";
import {readOfficialArticle, readOfficialList} from "@/lib/api/official-knowledge";
import "./knowledge.css";

const root = "/workbench/ai/knowledge/official";
const breadcrumbs = [{label: "AI工作台", href: "/workbench/ai"}, {label: "知识库", href: "/workbench/ai/knowledge"}, {label: "官方知识库", href: root}];
const description = "由硕米维护的应用指南，查看正文、版本和出处。AI 引用尚未开放。";
export function OfficialKnowledgePage({articleId, revision}: {articleId?: string; revision?: string}) {
  const context = useWorkbenchContext();
  if (context.error || context.blockingError) return <ConsoleState kind="error" title="企业上下文不可用"><Button onClick={() => void context.retry()}>重新确认</Button></ConsoleState>;
  if (context.isLoading || context.isSwitching) return <ConsoleState kind="loading" title="正在确认当前企业"/>;
  if (!context.user || !context.effectiveOrganization) return <ConsoleState kind="unavailable" title="请先选择企业"/>;
  if (!context.effectiveOrganization.permissions?.includes("workbench.knowledge.read")) return <ConsolePage title="官方知识库" description={description} breadcrumbs={breadcrumbs}><ConsoleState kind="unavailable" title="当前身份没有知识库读取权限"><Button onClick={() => void context.retry()}>重新确认</Button></ConsoleState></ConsolePage>;
  const scope = {userId: context.user.id, organizationId: context.effectiveOrganization.id};
  return <OfficialContent key={`${scope.userId}:${scope.organizationId}:${articleId ?? "list"}:${revision ?? ""}`} scope={scope} articleId={articleId} revision={revision}/>;
}
function authorityFailure(error: unknown) {
  return error instanceof KnowledgeError && [401, 403, 409].includes(error.status);
}
function message(error: unknown) {
  if (error instanceof KnowledgeError) {
    if (error.code === "OFFICIAL_KNOWLEDGE_NOT_FOUND") return "此版本的官方资料不存在";
    if (error.status === 403) return "当前身份没有知识库读取权限";
    if (error.status === 401) return "登录已失效，请重新确认";
    if (error.status === 409) return "身份或企业已变化，请重新确认";
  }
  return "官方资料暂时无法读取";
}
function OfficialContent({scope, articleId, revision}: {scope: KnowledgeScope; articleId?: string; revision?: string}) {
  const context = useWorkbenchContext(), client = useQueryClient();
  const [authorityError, setAuthorityError] = useState<unknown>(null);
  const key = useMemo(() => ["official-knowledge", scope.userId, scope.organizationId], [scope.userId, scope.organizationId]);
  useEffect(() => () => {void client.cancelQueries({queryKey: key}); client.removeQueries({queryKey: key});}, [client, key]);
  const read = async <T,>(run: () => Promise<T>) => {try {return await run();} catch (error) {if (authorityFailure(error)) setAuthorityError(error); throw error;}};
  const list = useQuery({queryKey: [...key, "list"], queryFn: ({signal}) => read(() => readOfficialList(scope, signal)), enabled: !articleId && !authorityError, retry: false, gcTime: 0});
  const detail = useQuery({queryKey: [...key, "article", articleId, revision], queryFn: ({signal}) => read(() => readOfficialArticle(scope, articleId!, revision!, signal)), enabled: !!articleId && !!revision && !authorityError, retry: false, gcTime: 0});
  const error = authorityError || (articleId ? detail.error : list.error);
  const refresh = async () => {const next = await context.retry(); if (next?.user.id === scope.userId && next.effectiveOrganizationId === scope.organizationId) {setAuthorityError(null); await (articleId ? detail.refetch() : list.refetch());}};
  return <ConsolePage title={!error && detail.data ? detail.data.title : "官方知识库"} description={description} breadcrumbs={articleId ? [...breadcrumbs, {label: "资料版本"}] : breadcrumbs}
    actions={articleId ? <Button asChild variant="outline"><Link href={root} prefetch={false}>返回官方知识库</Link></Button> : undefined}>
    {error ? <ConsoleState kind="error" title={message(error)}><Button onClick={() => void (authorityError ? refresh() : articleId ? detail.refetch() : list.refetch())}>{authorityError ? "重新确认" : "重新读取"}</Button></ConsoleState> : articleId ? detail.isPending ? <ConsoleState kind="loading" title="正在读取资料版本"/> : detail.data ? <>
      <Card className="official-knowledge-metadata"><p>{detail.data.maintainedBy}维护 · v{detail.data.revision} · 更新于 {detail.data.updatedAt}</p><p className="console-description">{detail.data.summary}</p><p className="official-knowledge-digest">SHA-256：{detail.data.digest}</p></Card>
      <Card className="knowledge-preview official-knowledge-article" role="region" aria-label="官方资料正文"><pre>{detail.data.body}</pre></Card>
      <Card className="official-knowledge-sources"><h2>内容出处</h2><ul>{detail.data.sources.map(source => <li key={source.url}><a href={source.url} target="_blank" rel="noopener noreferrer">{source.title}</a></li>)}</ul></Card>
    </> : null : list.isPending ? <ConsoleState kind="loading" title="正在读取官方资料"/> : list.data?.items.length === 0 ? <ConsoleState kind="empty" title="暂无已发布的官方资料"/> : <div className="knowledge-grid">{list.data?.items.map(item => <Card className="knowledge-entry-card" data-kind="official" key={item.id}>
      <span className="knowledge-status">{item.category}</span><h2>{item.title}</h2><p className="console-description">{item.summary}</p><p className="console-description">{item.maintainedBy}维护 · v{item.revision} · 更新于 {item.updatedAt}</p>
      <div className="knowledge-entry-actions"><Button asChild><Link href={`${root}/${item.id}/revisions/${item.revision}`} prefetch={false}>查看内容</Link></Button></div>
    </Card>)}</div>}
  </ConsolePage>;
}
