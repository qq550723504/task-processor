"use client";

import Link from "next/link";
import { useEffect,useMemo,useState } from "react";
import { useMutation,useQuery,useQueryClient } from "@tanstack/react-query";
import type { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { ConsolePage,ConsoleState } from "@/components/workbench/console/console-page";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { baseSchema,basesSchema,sourcesSchema,previewSchema,resultSchema,knowledgeRequest,KnowledgeError,type KnowledgeScope,type KnowledgeSource } from "@/lib/api/knowledge";
import "./knowledge.css";

const root="/workbench/ai/knowledge";
// Reserve 4 KiB for bounded source name, escaped filename, MIME and multipart
// boundaries. The server retains the 10 MiB whole-request contract.
const maxUploadFileBytes=10*1024*1024-4096;
const description="知识库归当前企业所有，默认不会自动用于 AI。支持 TXT、Markdown、可提取正文的 PDF 和 DOCX，不支持扫描件 OCR。";
export function KnowledgePage({baseId}:{baseId?:string}) {
 const context=useWorkbenchContext();
 if(context.isLoading) return <ConsoleState kind="loading" title="正在确认当前企业" />;
 if(context.error || context.blockingError) return <ConsoleState kind="error" title="企业上下文不可用"><Button onClick={()=>void context.retry()}>重新确认</Button></ConsoleState>;
 if(!context.user || !context.effectiveOrganization) return <ConsoleState kind="unavailable" title="请先选择企业" />;
 const scope={userId:context.user.id,organizationId:context.effectiveOrganization.id};
 // Switch preparation may be rejected by a pending-intent guard. Hide the old
 // view during preparation without discarding that intent; an actual identity
 // or organization change still remounts and clears all scoped state.
 return <>{context.isSwitching?<ConsoleState kind="loading" title="正在确认当前企业" />:null}<div hidden={context.isSwitching}><KnowledgeContent key={scope.userId+":"+scope.organizationId+":"+(baseId??"list")} scope={scope} baseId={baseId} /></div></>;
}
function errorMessage(error:unknown) {
 const code=error instanceof KnowledgeError?error.code:"KNOWLEDGE_UNAVAILABLE";
 const messages:Record<string,string>={
  KNOWLEDGE_SOURCE_LIMIT_REACHED:"已达到 4 份有效资料上限，请先停用一份资料。",
  KNOWLEDGE_REVISION_IN_PROGRESS:"这份资料已有版本正在处理，请稍后再替换。",
  KNOWLEDGE_CONFLICT:"资料已更新，或相同请求使用了不同内容。请刷新后重试。",
  KNOWLEDGE_DISABLED:"知识库或资料已停用。",
  KNOWLEDGE_NOT_READY:"暂无可预览的正文，请等待处理完成。",
  KNOWLEDGE_INVALID_REQUEST:"请检查名称、文件类型及大小，文件与请求总大小须小于 10 MiB。",
  PERMISSION_DENIED:"当前身份没有知识库管理或读取权限。",
  ORGANIZATION_CONTEXT_CHANGED:"企业已变化，请重新确认当前企业。",
  IDENTITY_CONTEXT_CHANGED:"登录身份已变化，请重新登录。",
  OUTCOME_UNKNOWN:"请求结果尚未确认。请使用下方按钮重试同一次操作，避免重复创建。",
 };
 return messages[code]??"暂时无法读取知识库，请稍后重试。";
}
const stateLabels:Record<string,string>={ADMITTED:"等待保存",OBJECT_STORED:"等待处理",PROCESSING:"处理中",AVAILABLE:"可用",PARTIAL:"部分提取",FAILED:"处理失败"};
const reasons:Record<string,string>={UPLOAD_INCOMPLETE:"上传未完成，请重试原上传。",OBJECT_INTEGRITY_FAILURE:"文件校验失败，请重新上传。",CORRUPT_OR_UNSUPPORTED_DOCUMENT:"文件损坏或无法解析，请重新上传。",DOCUMENT_TYPE_MISMATCH:"文件内容与格式不符。",NO_EXTRACTABLE_TEXT:"未提取到正文；扫描件不支持 OCR。",PARSER_RETRIES_EXHAUSTED:"解析服务多次失败，请重新上传。",PARSER_UNAVAILABLE:"解析服务暂时不可用。",TEXT_TRUNCATED:"正文超过 2 MiB，已保留前部分。",INCOMPLETE_EXTRACTION:"部分内容未能完整提取，请检查预览。"};
function date(value:string){return new Date(value).toLocaleString("zh-CN",{year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit"});}
type Intent={path:string;init:RequestInit;key:string;unconfirmed?:boolean};
function isAuthorityFailure(error:unknown):error is KnowledgeError{return error instanceof KnowledgeError && ([401,403].includes(error.status) || ["IDENTITY_CONTEXT_CHANGED","ORGANIZATION_CONTEXT_CHANGED"].includes(error.code));}
function KnowledgeContent({scope,baseId}:{scope:KnowledgeScope;baseId?:string}) {
 const client=useQueryClient(),context=useWorkbenchContext();
 // The verified current-organization roles include configured admin overrides.
 // Ordinary controls follow the same admin boundary as KnowledgeManage; the
 // server still authorizes every write with a fresh organization grant.
 const canManage=context.roles.some(role=>role==="listingkit_admin" || role==="platform_admin");
 const canRead=canManage || context.roles.includes("listingkit_operator");
 const [page,setPage]=useState(1),[name,setName]=useState(""),[editing,setEditing]=useState(false),[file,setFile]=useState<File|null>(null),[sourceName,setSourceName]=useState(""),[replacement,setReplacement]=useState<KnowledgeSource|null>(null),[preview,setPreview]=useState<KnowledgeSource|null>(null),[intent,setIntent]=useState<Intent|null>(null),[message,setMessage]=useState("");
 const key=["knowledge",scope.userId,scope.organizationId];
 const [authorityError,setAuthorityError]=useState<KnowledgeError|null>(null),[readAttempt,setReadAttempt]=useState(0);
 const rolesKey=JSON.stringify([...context.roles].sort());
 const readKey=useMemo(()=>["knowledge",scope.userId,scope.organizationId,"read",rolesKey,readAttempt],[scope.userId,scope.organizationId,rolesKey,readAttempt]);
 // Authorization changes create fresh read observers and cancel old in-flight
 // responses. The write intent remains scoped to the same identity/org.
 useEffect(()=>()=>{void client.cancelQueries({queryKey:readKey});client.removeQueries({queryKey:readKey});},[client,readKey]);
 // A query-key change must not erase an observed authorization failure. Keep
 // the fence until explicit context confirmation starts entirely fresh reads.
 useEffect(()=>{if(authorityError){void client.cancelQueries({queryKey:readKey});client.removeQueries({queryKey:readKey});}},[authorityError,client,readKey]);
 const read=async <T,>(path:string,schema:z.ZodType<T>,signal:AbortSignal)=>{try{return await knowledgeRequest(scope,path,schema,{signal});}catch(error){if(!signal.aborted && isAuthorityFailure(error))setAuthorityError(error);throw error;}};
 const readable=canRead && !authorityError;
 const list=useQuery({queryKey:[...readKey,"bases",page],queryFn:({signal})=>read("knowledge-bases?page="+page+"&pageSize=20",basesSchema,signal),enabled:readable && !baseId,retry:false,gcTime:0});
 const base=useQuery({queryKey:[...readKey,"base",baseId],queryFn:({signal})=>read("knowledge-bases/"+baseId,baseSchema,signal),enabled:readable && !!baseId,retry:false,gcTime:0,refetchInterval:5000});
 const sources=useQuery({queryKey:[...readKey,"sources",baseId],queryFn:async({signal})=>{
 const result=await read("knowledge-bases/"+baseId+"/sources",sourcesSchema,signal);
 if(result.items.some(source=>source.knowledgeBaseId!==baseId))throw new KnowledgeError("INVALID_UPSTREAM_RESPONSE");return result;
 },enabled:readable && !!baseId && base.data?.state==="ACTIVE",retry:false,gcTime:0,refetchInterval:5000});
 const currentPreview=!sources.error && preview ? sources.data?.items.find(s=>s.id===preview.id && s.state==="ACTIVE" && s.currentReadableRevision?.id===preview.currentReadableRevision?.id) : undefined;
 const text=useQuery({queryKey:[...readKey,"preview",currentPreview?.id,currentPreview?.currentReadableRevision?.id],queryFn:async({signal})=>{
 const revision=currentPreview!.currentReadableRevision!.id;
 const result=await read("knowledge-sources/"+currentPreview!.id+"/revisions/"+revision+"/preview",previewSchema,signal);
 if(result.revisionId!==revision)throw new KnowledgeError("INVALID_UPSTREAM_RESPONSE");return result;
 },enabled:readable && !!currentPreview && base.data?.state==="ACTIVE",retry:false,gcTime:0,staleTime:0,refetchInterval:5000});
 const mutation=useMutation({mutationKey:[...key,"mutation"],retry:false,mutationFn:(command:Intent)=>knowledgeRequest(scope,command.path,resultSchema,command.init),onSuccess:async(result)=>{
 setIntent(null);setMessage("操作已保存。");setEditing(false);setName("");setFile(null);setSourceName("");setReplacement(null);setPreview(null);
 if(result.knowledgeBase && baseId)client.setQueryData([...readKey,"base",baseId],result.knowledgeBase);
 await client.invalidateQueries({queryKey:key});
 },onError:(error,command)=>{const deniedRetry=command.unconfirmed && error instanceof KnowledgeError && [401,403,429].includes(error.status);const unknown=deniedRetry || error instanceof KnowledgeError && (error.code==="OUTCOME_UNKNOWN" || error.status>=500);setMessage(deniedRetry?errorMessage(error)+" 原请求结果仍未确认，请恢复访问后重试同一次操作。":errorMessage(unknown?new KnowledgeError("OUTCOME_UNKNOWN"):error));setIntent(unknown?{...command,unconfirmed:true}:null);void client.invalidateQueries({queryKey:key});}});
 const registerSwitchGuard=context.registerOrganizationSwitchGuard;
 useEffect(()=>registerSwitchGuard(()=>!mutation.isPending && !intent),[registerSwitchGuard,mutation.isPending,intent]);
 const busy=mutation.isPending || !!intent;
 const run=(path:string,method:string,body?:BodyInit,version?:number)=>{
 if(busy || !canManage)return;const id=crypto.randomUUID();const headers:Record<string,string>={"Idempotency-Key":id};if(version)headers["If-Match"]='"'+version+'"';if(typeof body==="string")headers["Content-Type"]="application/json";
 const command={path,init:{method,headers,body},key:id};setIntent(command);setMessage("");mutation.mutate(command);
 };
 const selectedCount=sources.data?.items.filter(s=>s.state==="ACTIVE").length??0;
 const upload=()=>{
 if(!file || !baseId || file.size===0 || file.size>maxUploadFileBytes)return;
 const body=new FormData();body.set("name",(replacement?.name??sourceName).trim());body.set("file",file);
 run(replacement?"knowledge-sources/"+replacement.id+"/revisions":"knowledge-bases/"+baseId+"/sources","POST",body,replacement?.version);
 };
 const active=base.data?.state==="ACTIVE";
 const listError=!baseId && list.error, detailError=baseId && base.error;
 const notice=message?<Card className="knowledge-notice" role={mutation.isError?"alert":"status"}><p>{message}</p>{intent && !mutation.isPending && readable?<Button onClick={()=>mutation.mutate(intent)}>重试同一次操作</Button>:null}</Card>:null;
 const refreshAccess=async()=>{const next=await context.retry();if(next?.user.id===scope.userId && next.effectiveOrganizationId===scope.organizationId){setReadAttempt(value=>value+1);setAuthorityError(null);}};
 if(!readable)return <ConsolePage title="我的知识库（当前企业）" description={description}>{notice}<ConsoleState kind="unavailable" title={canRead?errorMessage(authorityError):"当前身份没有知识库读取权限"}><Button onClick={()=>void refreshAccess()}>重新确认</Button></ConsoleState></ConsolePage>;
 return <ConsolePage title={baseId?(base.data?.name??"知识库"):"我的知识库（当前企业）"} description={description} breadcrumbs={baseId?[{label:"知识库",href:root},{label:base.data?.name??"资料"}]:undefined}
 actions={canManage?(!baseId?<Button onClick={()=>setEditing(true)} disabled={busy}>创建知识库</Button>:active?<Button variant="outline" disabled={busy} onClick={()=>{setName(base.data!.name);setEditing(true);}}>编辑名称</Button>:undefined):undefined}>
 {notice}
 {editing && canManage?<Card className="knowledge-form"><form onSubmit={e=>{e.preventDefault();run(baseId?"knowledge-bases/"+baseId:"knowledge-bases",baseId?"PUT":"POST",JSON.stringify({name}),base.data?.version);}}>
 <label htmlFor="knowledge-name">知识库名称</label><Input id="knowledge-name" value={name} onChange={e=>setName(e.target.value)} required disabled={busy} placeholder="例如：品牌资料" />
 <div className="knowledge-actions"><Button type="submit" disabled={busy || !name.trim() || [...name.trim()].length>120}>保存</Button><Button variant="outline" disabled={busy} onClick={()=>setEditing(false)}>取消</Button></div></form></Card>:null}
 {listError || detailError?<ConsoleState kind="error" title={errorMessage(listError||detailError)}><Button onClick={()=>void (baseId?base.refetch():list.refetch())}>重新读取</Button></ConsoleState>:null}
 {!baseId?<>
 {list.isPending?<ConsoleState kind="loading" title="正在读取知识库" />:null}
 {list.data?.items.length===0?<ConsoleState kind="empty" title="当前企业还没有知识库">{canManage?"创建知识库后上传企业资料，处理完成后即可预览正文。":"管理员创建知识库并上传企业资料后，即可在此预览正文。"}</ConsoleState>:null}
 <div className="knowledge-grid">{list.data?.items.map(item=><Card className="knowledge-base-card" key={item.id}><h2>{item.name}</h2><p className="console-description">更新于 {date(item.updatedAt)}</p><p className="console-description">更新人：{item.updatedBy}</p><div className="knowledge-actions"><span className="knowledge-status" data-state={item.state}>{item.state==="ACTIVE"?"启用中":"已停用"}</span><Button asChild><Link href={root+"/"+item.id} prefetch={false}>打开知识库</Link></Button></div></Card>)}</div>
 {list.data?<div className="knowledge-pagination"><Button variant="outline" disabled={page===1} onClick={()=>setPage(v=>v-1)}>上一页</Button><span>第 {page} 页 · 共 {list.data.pagination.total} 个</span><Button variant="outline" disabled={page*20>=list.data.pagination.total} onClick={()=>setPage(v=>v+1)}>下一页</Button></div>:null}
 </>:<>
 {base.isPending?<ConsoleState kind="loading" title="正在读取知识库" />:null}
 {base.data && !active?<ConsoleState kind="unavailable" title="知识库已停用">资料正文已停止读取。</ConsoleState>:null}
 {active?<Card className="knowledge-sources"><div className="knowledge-panel-header"><div><h2>资料与处理状态</h2><p className="console-description">有效资料 {selectedCount}/4 · 只预览当前可读版本</p></div>{canManage?<div className="knowledge-actions"><Button variant="outline" disabled={busy} onClick={()=>{if(window.confirm("停用知识库后将无法读取资料正文，且无法恢复。是否继续？"))run("knowledge-bases/"+baseId+"/disable","POST",undefined,base.data!.version);}}>停用知识库</Button><Button disabled={busy || selectedCount>=4 || sources.isPending || !!sources.error} onClick={()=>{setReplacement(null);setFile(null);setEditing(false);document.getElementById("knowledge-upload")?.focus();}}>上传资料</Button></div>:null}</div>
 {canManage?<div className="knowledge-upload"><label htmlFor="knowledge-upload">{replacement?"替换："+replacement.name:"上传新资料"}</label><input id="knowledge-upload" type="file" accept=".txt,.md,.markdown,.pdf,.docx" disabled={busy || (!replacement && selectedCount>=4)} onChange={e=>{const selected=e.target.files?.[0]??null;setFile(selected);if(!replacement)setSourceName([...selected?.name??""].slice(0,120).join(""));}} />
 {!replacement?<><label htmlFor="knowledge-source-name">资料名称</label><Input id="knowledge-source-name" disabled={busy} value={sourceName} onChange={e=>setSourceName(e.target.value)} placeholder="最多 120 个字符" /></>:null}
 <span className="console-description">文件与请求总大小不超过 10 MiB，每次一份资料。</span>{file && file.size>maxUploadFileBytes?<p className="knowledge-reason" role="alert">文件接近大小上限，请缩小一点后上传。</p>:null}<div className="knowledge-actions"><Button disabled={busy || !file || file.size===0 || file.size>maxUploadFileBytes || !replacement && (!sourceName.trim() || [...sourceName.trim()].length>120)} onClick={upload}>{replacement?"提交新版本":"上传并处理"}</Button>{replacement?<Button variant="outline" disabled={busy} onClick={()=>{setReplacement(null);setFile(null);}}>取消替换</Button>:null}</div></div>:null}
 {sources.isPending?<ConsoleState kind="loading" title="正在读取资料" />:sources.error?<ConsoleState kind="error" title={errorMessage(sources.error)}><Button onClick={()=>void sources.refetch()}>重新读取</Button></ConsoleState>:sources.data?.items.length===0?<ConsoleState kind="empty" title="尚未上传资料" />:null}
 <div className="knowledge-source-list">{sources.data?.items.map(source=>{
 const latest=source.latestRevision,readable=source.currentReadableRevision,isActive=source.state==="ACTIVE",processing=latest && ["ADMITTED","OBJECT_STORED","PROCESSING"].includes(latest.state);
 return <article className="knowledge-source" key={source.id}><div className="knowledge-source-copy"><h3>{isActive?source.name:"已停用资料"}</h3>{isActive && latest?<><p className="console-description">最新版本 v{latest.number} · {latest.filename} · {date(latest.updatedAt)}</p>{readable && readable.id!==latest.id?<p className="console-description">当前仍可读取 v{readable.number}，新版本尚未替换旧正文。</p>:null}<span className="knowledge-status" data-state={latest.state}>{stateLabels[latest.state]}</span>{latest.failure || latest.warning?<p className="knowledge-reason">{reasons[latest.failure??latest.warning??""]??"文件处理未完成，请检查资料后重试。"}</p>:null}</>:null}</div><div className="knowledge-actions">{isActive?<><Button variant="outline" disabled={!readable} onClick={()=>setPreview(source)}>预览{readable && latest && readable.id!==latest.id?"旧版本":""}</Button>{canManage?<><Button variant="outline" disabled={busy || !!processing} onClick={()=>{setReplacement(source);setFile(null);document.getElementById("knowledge-upload")?.focus();}}>重新上传</Button><Button variant="outline" disabled={busy} onClick={()=>{if(window.confirm("停用后将无法读取此资料正文，且无法恢复。是否继续？"))run("knowledge-sources/"+source.id+"/disable","POST",undefined,source.version);}}>停用</Button></>:null}</>:<span className="knowledge-status">已停用</span>}</div></article>;
 })}</div></Card>:null}
 {active && currentPreview?<Card className="knowledge-preview" role="region" aria-label="资料正文预览"><div className="knowledge-panel-header"><h2>{currentPreview.name} · v{currentPreview.currentReadableRevision!.number}</h2><Button variant="outline" onClick={()=>setPreview(null)}>关闭预览</Button></div>{text.isPending?<p role="status">正在读取正文</p>:text.error?<p role="alert">{errorMessage(text.error)}</p>:text.data?<>{text.data.warning?<p className="knowledge-reason">{reasons[text.data.warning]??"正文部分提取，请检查内容。"}</p>:null}<pre>{text.data.text}</pre></>:null}</Card>:null}
 </>}
 </ConsolePage>;
}
