"use client";
import Link from "next/link";
import { FolderKanban } from "lucide-react";
import {knowledgeRequest,sourcesSchema,type KnowledgeSource} from "@/lib/api/knowledge";
import { useEffect,useRef,useState } from "react";
import { useRouter } from "next/navigation";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { ConsolePage,ConsoleState } from "../console/console-page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { kinds,page,project,templates,receipt,projectBody,projectEndpoint,referenceFromLink,type Project,type Template,type ProjectBody,type Reference } from "@/lib/contracts/project-center";
import { projectRequest,ProjectError,type Intent,type ProjectScope } from "@/lib/api/project-center";
import styles from "./project-page.module.css";

type Mode="active"|"recent"|"archived"|"templates";
const root="/workbench/ai/projects";
const descriptions:Record<Mode,string>={active:"管理长期目标，关联已有会话、任务和资料。",recent:"继续查看最近访问过的项目。",archived:"查看已归档项目，保留目标与关联内容。",templates:"把自己的项目目标保存为模板，创建时可重新调整。"};
const titles:Record<Mode,string>={active:"进行中项目",recent:"最近访问",archived:"已归档项目",templates:"项目模板"};
const messages:Record<string,string>={OUTCOME_UNKNOWN:"操作结果暂无法确认，请重试原操作。",REVISION_MISMATCH:"项目已更新，请刷新后重新操作。",PROJECT_ARCHIVED:"项目已归档，请先恢复。",NOT_FOUND:"内容不存在或当前不可访问。",FORBIDDEN:"当前权限不可用，请重新确认企业。",IDENTITY_CONTEXT_CHANGED:"登录身份已变化，请刷新。",ORGANIZATION_CONTEXT_CHANGED:"企业已变化，请重新确认。",IDEMPOTENCY_CONFLICT:"操作内容与原请求不一致，请刷新。",INVALID_REQUEST:"请检查项目名称、目标、类型和截止日期。"};
function errorText(e:unknown){return e instanceof ProjectError?messages[e.code]??"服务暂不可用，请稍后重试。":"服务暂不可用，请稍后重试。";}
const terminalRejections=new Set(["REVISION_MISMATCH","PROJECT_ARCHIVED","NOT_FOUND","INVALID_REQUEST","IDEMPOTENCY_CONFLICT"]);
export function ProjectPage({mode="active",projectId}:{mode?:Mode;projectId?:string}){
 const c=useWorkbenchContext();const ready=c.user && c.effectiveOrganization && !c.isLoading && !c.isSwitching && !c.selectionRequired && !c.error && !c.blockingError;
 const scope=ready?{userId:c.user!.id,organizationId:c.effectiveOrganization!.id}:null;
 const read=c.permissions.includes("workbench.project.read"),manage=c.permissions.includes("workbench.project.manage");
 return <ConsolePage title={projectId?"项目详情":titles[mode]} description={projectId?"目标和关联内容仅由你在当前企业内查看。":descriptions[mode]} breadcrumbs={[{label:"AI工作台"},{label:"项目中心",href:root},{label:projectId?"项目详情":titles[mode]}]} className={styles.page}>
 {!scope?<ConsoleState kind={c.isLoading || c.isSwitching?"loading":"unavailable"} title="请先确认当前企业"/>:!c.projectCenterAvailable?<ConsoleState kind="unavailable" title="当前应用暂未启用项目中心"/>:!read?<ConsoleState kind="unavailable" title="当前身份没有项目查看权限"/>:<ProjectContent key={JSON.stringify([scope,c.permissions,mode,projectId])} scope={scope} mode={mode} projectId={projectId} manage={manage}/>}
 </ConsolePage>;
}
function summary(p:Project){return !p.taskSummaryAvailable?"任务状态暂不可用":p.taskTotal===0?"尚未关联任务":`${p.taskCompleted} / ${p.taskTotal} 项关联任务已完成`;}
function referenceTitle(r:Reference){return r.available?r.title:"内容当前不可访问";}
function ProjectContent({scope,mode,projectId,manage}:{scope:ProjectScope;mode:Mode;projectId?:string;manage:boolean}){
 const router=useRouter();const [items,setItems]=useState<Project[]>([]),[saved,setSaved]=useState<Template[]>([]),[detail,setDetail]=useState<Project|null>(null),[next,setNext]=useState(""),[after,setAfter]=useState(""),[search,setSearch]=useState(""),[kind,setKind]=useState(""),[workScope,setWorkScope]=useState(""),[loading,setLoading]=useState(true),[error,setError]=useState(""),[refresh,setRefresh]=useState(0),[busy,setBusy]=useState(false),[pending,setPending]=useState<Intent|null>(null),[hydrated,setHydrated]=useState(false),[storageError,setStorageError]=useState(""),[form,setForm]=useState<{body:ProjectBody;edit?:Project}|null>(null),[tab,setTab]=useState("项目概览"),[link,setLink]=useState(""),[templateName,setTemplateName]=useState(""),[sources,setSources]=useState<KnowledgeSource[]>([]);
 const lifetime=useRef<AbortController|null>(null);const storageKey=`project-center:intent:${scope.userId}:${scope.organizationId}`;
 useEffect(()=>{const controller=new AbortController();lifetime.current=controller;
  void Promise.resolve().then(()=>{if(controller.signal.aborted)return;try{const raw=sessionStorage.getItem(storageKey);if(raw){const intent=JSON.parse(raw) as Intent;const endpoint=projectEndpoint(new URL("/api/workbench/projects"+intent.path,location.origin),intent.method);if(endpoint?.input && endpoint.input.safeParse(intent.body).success && /^[0-9a-f-]{36}$/.test(intent.key) && (!endpoint.expected || Number.isSafeInteger(intent.revision) && intent.revision!>0))setPending(intent);else setStorageError("原操作记录无法读取，请保留此页面并重新确认。");}}catch{setStorageError("无法读取原操作记录，请允许浏览器会话存储后刷新。");}setHydrated(true);});
  return ()=>controller.abort();
 },[storageKey]);
 useEffect(()=>{const controller=new AbortController();
  const load=async()=>{await Promise.resolve();if(controller.signal.aborted)return;setLoading(true);setError("");try{
   if(projectId){const p=await projectRequest(scope,"/"+projectId,project,controller.signal);if(controller.signal.aborted)return;setDetail(p);setLoading(false);}
   else if(mode==="templates"){const p=await projectRequest(scope,"/templates"+(after?"?after="+encodeURIComponent(after):""),templates,controller.signal);if(controller.signal.aborted)return;setSaved(old=>after?[...old,...p.templates]:p.templates);setNext(p.next);setLoading(false);}
   else{const query=new URLSearchParams({mode,search,kind,workScope,after});const p=await projectRequest(scope,"?"+query,page,controller.signal);if(controller.signal.aborted)return;setItems(old=>after?[...old,...p.projects]:p.projects);setNext(p.next);setLoading(false);}
  }catch(e){if(!controller.signal.aborted){setError(errorText(e));setLoading(false);}}};void load();return ()=>controller.abort();
 },[scope,mode,projectId,after,search,kind,workScope,refresh]);
 const reload=()=>{setAfter("");setRefresh(v=>v+1);};
 const mutate=async(intent:Intent,isVisit=false)=>{
  const controller=lifetime.current;if(!controller || controller.signal.aborted || busy || !hydrated)return;
  const recovering=pending?.key===intent.key;
  if(!isVisit){try{sessionStorage.setItem(storageKey,JSON.stringify(intent));setPending(intent);}catch{setError("请允许浏览器会话存储，以保留操作结果未知时的重试记录。");return;}}
  setBusy(true);setError("");
  try{const r=await projectRequest(scope,intent.path,receipt,controller.signal,intent);if(controller.signal.aborted)return;
   if(!isVisit){sessionStorage.removeItem(storageKey);setPending(null);setForm(null);setLink("");setTemplateName("");setSources([]);if(intent.path==="" && intent.method==="POST")router.push(root+"/"+r.id);else reload();}
  }catch(e){if(controller.signal.aborted)return;setForm(null);setError(errorText(e));if(e instanceof ProjectError && e.code!=="OUTCOME_UNKNOWN" && (!recovering || terminalRejections.has(e.code)) && !isVisit){sessionStorage.removeItem(storageKey);setPending(null);}}
  finally{if(!controller.signal.aborted)setBusy(false);}
 };
 // Visits are explicit commands and never modify the project's business revision.
 useEffect(()=>{if(!projectId || !hydrated)return;const controller=new AbortController();const intent:Intent={path:"/"+projectId+"/visit",method:"POST",body:{},key:crypto.randomUUID()};void projectRequest(scope,intent.path,receipt,controller.signal,intent).catch(()=>{});return ()=>controller.abort();},[scope,projectId,hydrated]);
 const readMaterials=async()=>{const r=referenceFromLink(link),controller=lifetime.current;if(!controller || r?.kind!=="KNOWLEDGE_BASE"){setError("先粘贴知识库详情链接，再选择其中的资料。");return;}try{const result=await knowledgeRequest(scope,"knowledge-bases/"+r.targetId+"/sources",sourcesSchema,{signal:controller.signal});if(controller.signal.aborted)return;setSources(result.items.filter(item=>item.state==="ACTIVE"));}catch(e){if(!controller.signal.aborted)setError(errorText(e));}};
 const command=(path:string,body:unknown={},revision?:number,method:"POST"|"PATCH"="POST")=>void mutate({path,body,revision,method,key:crypto.randomUUID()});
 const disabled=busy || !!pending || !hydrated || !!storageError;
 const newProject=(template?:Template)=>setForm({body:{title:template?.title??"",goal:template?.goal??"",kind:template?.kind??"OTHER",dueDate:""}});

 return <>
 <nav className={styles.tabs} aria-label="项目分类">{(Object.keys(titles) as Mode[]).map(v=><Link href={root+"/"+v} key={v} aria-current={!projectId && v===mode?"page":undefined}>{titles[v]}</Link>)}</nav>
 {pending?<Card className={styles.notice} role="status"><p>有一项操作需要确认结果，重试会读取同一操作的收据。</p><Button disabled={busy || !hydrated} onClick={()=>void mutate(pending)}>重试原操作</Button></Card>:null}
 {storageError?<ConsoleState kind="error" title={storageError}/>:null}
 {error?<ConsoleState kind="error" title={error}><Button variant="outline" disabled={busy} onClick={reload}>刷新</Button></ConsoleState>:null}
 {!projectId?<div className={styles.toolbar}><div><h2>{titles[mode]}</h2><p>{mode==="templates"?"仅保存名称、目标和项目类型，创建时重新选择店铺与资料。":"个人项目仅你可见"}</p></div>{manage?<Button disabled={disabled} onClick={()=>newProject()}>＋ 新建项目</Button>:null}</div>:null}
 {!projectId && mode!=="templates"?<div className={styles.filters}><Input aria-label="搜索项目" placeholder="搜索项目名称" value={search} onChange={e=>{setAfter("");setSearch(e.target.value);}}/><select aria-label="项目类型" value={kind} onChange={e=>{setAfter("");setKind(e.target.value);}}><option value="">全部类型</option>{Object.entries(kinds).map(([k,v])=><option key={k} value={k}>{v}</option>)}</select><select aria-label="项目范围" value={workScope} onChange={e=>{setAfter("");setWorkScope(e.target.value);}}><option value="">全部项目</option><option value="store">店铺项目</option><option value="general">非店铺项目</option></select></div>:null}
 {loading?<ConsoleState kind="loading" title="正在读取项目"/>:null}
 {!loading && !error && !projectId && (mode==="templates"?saved.length===0:items.length===0)?<ConsoleState kind="empty" title={mode==="templates"?"还没有个人模板":"还没有项目"}>{mode==="templates"?"打开已有项目，选择保存为模板。":"保存一个长期目标，开始整理相关工作。"}</ConsoleState>:null}
 {!projectId && mode!=="templates"?<div className={styles.grid}>{items.map(p=><Card key={p.id} className={styles.card}><div className={styles.cardHead}><span className={styles.icon}><FolderKanban aria-hidden="true" size={22}/></span><span className={styles.badge}>{p.archived?"已归档":"进行中"}</span></div><h3>{p.title}</h3><p className={styles.meta}>{kinds[p.kind]} · {p.storeScope?p.store?.available?p.store.title:"店铺当前不可访问":"非店铺项目"}</p><p className={styles.goal}>{p.goal}</p><p>{summary(p)}</p>{p.taskSummaryAvailable && p.taskTotal>0?<progress value={p.taskCompleted} max={p.taskTotal} aria-label="关联任务完成情况"/>:null}<div className={styles.cardFoot}><span>{p.dueDate?"截止 "+p.dueDate:"未设截止日期"}</span><Button variant="outline" size="sm" asChild><Link href={root+"/"+p.id} prefetch={false}>查看项目</Link></Button></div></Card>)}</div>:null}
 {!projectId && mode==="templates"?<div className={styles.grid}>{saved.map(t=><Card key={t.id} className={styles.card}><span className={styles.icon}><FolderKanban aria-hidden="true" size={22}/></span><h3>{t.name}</h3><p className={styles.meta}>{kinds[t.kind]}</p><p className={styles.goal}>{t.goal}</p>{manage?<div className={styles.cardFoot}><Button disabled={disabled} onClick={()=>newProject(t)}>使用模板</Button><Button variant="outline" disabled={disabled} onClick={()=>command("/templates/"+t.id+"/archive",{},t.revision)}>移除模板</Button></div>:null}</Card>)}</div>:null}
 {next && !projectId?<Button variant="outline" disabled={loading} onClick={()=>setAfter(next)}>加载更多</Button>:null}
 {detail && !loading?<Card className={styles.detail}><div className={styles.toolbar}><div><h2>{detail.title}</h2><p>{kinds[detail.kind]} · {detail.archived?"已归档":"进行中"}</p></div>{manage?<div className={styles.actions}><Button disabled={disabled || detail.archived} variant="outline" onClick={()=>setForm({body:{title:detail.title,goal:detail.goal,kind:detail.kind,dueDate:detail.dueDate},edit:detail})}>编辑项目</Button><Button disabled={disabled} variant="outline" onClick={()=>command("/"+detail.id+(detail.archived?"/restore":"/archive"),{},detail.revision)}>{detail.archived?"恢复项目":"归档项目"}</Button></div>:null}</div>
 <div className={styles.goalPanel}><small>项目目标</small><p>{detail.goal}</p>{detail.dueDate?<small>截止日期 {detail.dueDate}</small>:null}</div>
 <div className={styles.tabs} role="group" aria-label="项目内容">{["项目概览","任务","资料","项目知识","我的报告"].map(v=><Button key={v} variant="outline" aria-pressed={tab===v} onClick={()=>setTab(v)}>{v}</Button>)}</div>
 {tab==="项目概览"?<div className={styles.overview}><h3>关联任务进展</h3><p>{summary(detail)}</p>{detail.taskSummaryAvailable && detail.taskTotal>0?<progress value={detail.taskCompleted} max={detail.taskTotal} aria-label="关联任务完成情况"/>:null}<div className={styles.metrics}><Card><small>待确认任务</small><strong>{detail.taskSummaryAvailable?detail.taskPending:"暂不可用"}</strong></Card><Card><small>关联内容</small><strong>{detail.references.length}</strong></Card><Card><small>最近更新</small><strong>{new Date(detail.updatedAt).toLocaleDateString()}</strong></Card></div><h3>关联会话</h3>{<ReferenceList references={detail.references.filter(r=>r.kind==="CONVERSATION")} canRemove={manage && !detail.archived} disabled={disabled} remove={slot=>command(`/${detail.id}/references/${slot}/remove`,{},detail.revision)}/>}</div>:tab==="任务"?<ReferenceList references={detail.references.filter(r=>r.kind==="BUSINESS_TASK")} canRemove={manage && !detail.archived} disabled={disabled} remove={slot=>command(`/${detail.id}/references/${slot}/remove`,{},detail.revision)}/>:tab==="资料"?<ReferenceList references={detail.references.filter(r=>r.kind==="PRODUCT" || r.kind==="KNOWLEDGE_SOURCE")} canRemove={manage && !detail.archived} disabled={disabled} remove={slot=>command(`/${detail.id}/references/${slot}/remove`,{},detail.revision)}/>:tab==="项目知识"?<ReferenceList references={detail.references.filter(r=>r.kind==="KNOWLEDGE_BASE")} canRemove={manage && !detail.archived} disabled={disabled} remove={slot=>command(`/${detail.id}/references/${slot}/remove`,{},detail.revision)}/>:<div className={styles.refs}>{detail.references.filter(r=>r.resultHref).length?detail.references.filter(r=>r.resultHref).map(r=><Link key={r.slotId} href={r.resultHref!} prefetch={false}>{r.title} · 查看任务成果</Link>):<p>暂无可读取的关联任务成果。</p>}</div>}
 {!detail.archived && manage?<form className={styles.linkForm} onSubmit={e=>{e.preventDefault();const r=referenceFromLink(link);if(!r){setError("请粘贴当前应用的会话、任务、采集结果或知识库链接。");return;}command("/"+detail.id+"/references",r,detail.revision);}}><label>关联已有内容<Input value={link} onChange={e=>setLink(e.target.value)} placeholder="粘贴会话、任务、采集结果或知识库链接" required/></label><Button type="submit" disabled={disabled}>添加关联</Button><Button variant="outline" disabled={disabled} onClick={()=>void readMaterials()}>选择知识资料</Button></form>:null}
 {sources.length>0 && detail && !detail.archived?<div className={styles.refs}>{sources.map(item=><div className={styles.row} key={item.id}><span>{item.name}</span><Button size="sm" disabled={disabled} onClick={()=>command("/"+detail.id+"/references",{kind:"KNOWLEDGE_SOURCE",targetId:item.id},detail.revision)}>关联资料</Button></div>)}</div>:null}
 {manage && !detail.archived?<form className={styles.linkForm} onSubmit={e=>{e.preventDefault();command("/templates",{projectId:detail.id,name:templateName},detail.revision);}}><label>保存为个人模板<Input aria-label="模板名称" value={templateName} onChange={e=>setTemplateName(e.target.value)} placeholder="模板名称" required/></label><Button type="submit" disabled={disabled}>保存为模板</Button></form>:null}
 </Card>:null}
 {form?<ProjectForm initial={form.body} edit={form.edit} disabled={disabled} close={()=>setForm(null)} save={body=>command(form.edit?"/"+form.edit.id:"",body,form.edit?.revision,form.edit?"PATCH":"POST")}/>:null}
 </>;
}
function ProjectForm({initial,edit,disabled,close,save}:{initial:ProjectBody;edit?:Project;disabled:boolean;close:()=>void;save:(body:ProjectBody)=>void}){
 const [body,setBody]=useState(initial),[store,setStore]=useState(""),[changeStore,setChangeStore]=useState(false),[error,setError]=useState("");const dialog=useRef<HTMLDialogElement>(null);
 useEffect(()=>{const d=dialog.current;d?.showModal();return ()=>{d?.close();};},[]);
 const field=(key:keyof ProjectBody,value:string)=>setBody(old=>({...old,[key]:value}));
 return <dialog ref={dialog} className={styles.modal} onCancel={e=>{if(disabled)e.preventDefault();else close();}} aria-labelledby="project-form-title"><form onSubmit={e=>{e.preventDefault();const value={...body};if(changeStore || !edit){if(store.trim()){try{const u=new URL(store,location.origin),m=u.pathname.match(/^\/workbench\/stores\/([A-Za-z0-9._:-]+)$/);if(u.origin!==location.origin || !m || u.search || u.hash)throw new Error();value.storeId=m[1];}catch{setError("请粘贴当前应用的店铺详情链接。");return;}}else value.storeId="";}const parsed=projectBody.safeParse(value);if(!parsed.success){setError("请填写项目名称、长期目标，并检查日期和内容长度。");return;}save(parsed.data);}}>
 <header><h2 id="project-form-title">{edit?"编辑项目":"新建项目"}</h2><Button variant="ghost" aria-label="关闭" disabled={disabled} onClick={close}>×</Button></header><p>保存长期目标，再关联已有会话、任务与资料。</p>
 <label>项目名称<Input value={body.title} onChange={e=>field("title",e.target.value)} required placeholder="例如：一号店经营提升" autoFocus maxLength={256}/></label>
 <label>项目目标<textarea value={body.goal} onChange={e=>field("goal",e.target.value)} required rows={4} maxLength={4096} placeholder="希望达成什么长期目标？"/></label>
 <div className={styles.formGrid}><label>项目类型<select value={body.kind} onChange={e=>field("kind",e.target.value)}>{Object.entries(kinds).map(([k,v])=><option key={k} value={k}>{v}</option>)}</select></label><label>截止日期（可选）<Input type="date" value={body.dueDate} onChange={e=>field("dueDate",e.target.value)}/></label></div>
 {edit?<label><input type="checkbox" checked={changeStore} onChange={e=>setChangeStore(e.target.checked)}/> 更改关联店铺（留空改为非店铺项目）</label>:null}
 {!edit || changeStore?<label>关联店铺（可选）<Input value={store} onChange={e=>setStore(e.target.value)} placeholder="粘贴店铺详情链接；留空为非店铺项目"/></label>:null}
 {error?<p role="alert">{error}</p>:null}<footer><Button disabled={disabled} variant="outline" onClick={close}>取消</Button><Button disabled={disabled} type="submit">{edit?"保存修改":"创建项目"}</Button></footer>
 </form></dialog>;
}

function ReferenceList({references,canRemove,disabled,remove}:{references:Reference[];canRemove:boolean;disabled:boolean;remove:(slot:string)=>void}){return <div className={styles.refs}>{references.length===0?<p>尚未关联内容。</p>:references.map(r=><div className={styles.row} key={r.slotId}><div>{r.available?<Link prefetch={false} href={r.href!}>{referenceTitle(r)}</Link>:<span>{referenceTitle(r)}</span>}{r.taskState?<small>{r.taskState}</small>:null}</div>{canRemove?<Button size="sm" variant="outline" disabled={disabled} onClick={()=>remove(r.slotId)}>移除关联</Button>:null}</div>)}</div>;}
