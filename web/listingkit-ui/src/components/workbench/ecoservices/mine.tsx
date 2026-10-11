"use client";
import Link from "next/link";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {ConsolePage,ConsoleState} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {ecoRequest,ecoPageSchema,type EcoScope} from "@/lib/api/ecoservices";
import {EcoBoundary,EcoPagination,ReadFailure,categories,serviceStates,financialStates,date,yuan,NonPaymentNotice} from "./shared";
import {RequestDetail} from "./request-detail";
export function EcoservicesMine({nonPaymentOnly=false}:{nonPaymentOnly?:boolean}={}){return <EcoBoundary>{scope=><Mine key={scope.userId+scope.organizationId} scope={scope} nonPaymentOnly={nonPaymentOnly}/>}</EcoBoundary>}
function Mine({scope,nonPaymentOnly}:{scope:EcoScope;nonPaymentOnly:boolean}){
 const context=useWorkbenchContext(),[page,setPage]=useState(1),[search,setSearch]=useState(""),[category,setCategory]=useState(""),[side,setSide]=useState("buyer"),[stage,setStage]=useState(""),[from,setFrom]=useState(""),[to,setTo]=useState(""),[selected,setSelected]=useState("");
 const canRead=context.permissions.includes("workbench.ecoservices.read"),canManage=context.permissions.includes("workbench.ecoservices.manage");
 const params=new URLSearchParams({page:String(page),pageSize:"20",side});if(search)params.set("search",search);if(category)params.set("category",category);if(stage)params.set("stage",stage);if(from)params.set("from",new Date(from+"T00:00:00").toISOString());if(to)params.set("to",new Date(to+"T23:59:59.999").toISOString());
 const query=useQuery({queryKey:["ecoservices",scope.userId,scope.organizationId,"requests",params.toString(),context.permissions.join("|")],queryFn:({signal})=>ecoRequest(scope,"requests?"+params,ecoPageSchema,{signal}),enabled:canRead});
 const c=query.data?.counts,count=(states:string[])=>query.data?states.reduce((n,s)=>n+(c?.[s]??0),0):"—";
 return <ConsolePage title="我的服务" className="eco-page" description="跟进已提交需求、服务进度、交付验收与售后处理。" breadcrumbs={[{label:"生态服务"},{label:"我的服务"}]} actions={<Link href="/workbench/services/market">前往服务市场 →</Link>}>
  {nonPaymentOnly?<NonPaymentNotice/>:null}
  {!canRead?<ConsoleState kind="unavailable" title="当前身份没有服务读取权限"/>:<>
   <div className="eco-stats">{[["pending","待处理",count(["REQUESTED","QUOTED","ORDER_PENDING","PAID_READY"])],["servicing","服务中",count(["SERVICING"])],["acceptance","待验收",count(["AWAITING_ACCEPTANCE"])],["completed","已验收",count(["ACCEPTED"])]].map(([id,label,value])=><button className="eco-stat" key={id} aria-pressed={stage===id} onClick={()=>{setStage(stage===id?"":String(id));setPage(1)}}><span>{label}</span><strong>{value}</strong></button>)}</div>
   <Card className="eco-filter"><label>服务名称<Input maxLength={200} placeholder="搜索已提交服务" value={search} onChange={e=>{setSearch(e.target.value);setPage(1)}}/></label><label>服务类型<select value={category} onChange={e=>{setCategory(e.target.value);setPage(1)}}><option value="">全部类型</option>{categories.map(c=><option key={c.id} value={c.id}>{c.name}</option>)}</select></label><label>当前身份<select value={side} onChange={e=>{setSide(e.target.value);setPage(1)}}><option value="buyer">我购买的服务</option>{canManage?<option value="provider">我提供的服务</option>:null}</select></label><label>提交日期起<input type="date" value={from} onChange={e=>{setFrom(e.target.value);setPage(1)}}/></label><label>提交日期止<input type="date" value={to} min={from} onChange={e=>{setTo(e.target.value);setPage(1)}}/></label><Button variant="outline" onClick={()=>{setSearch("");setCategory("");setStage("");setFrom("");setTo("");setPage(1)}}>重置筛选</Button></Card>
   {query.isPending?<ConsoleState kind="loading" title="正在读取已保存服务"/>:query.error?<ReadFailure error={query.error} retry={()=>void query.refetch()}/>:query.data?.requests?.length?<Card className="eco-table-card"><div className="eco-table-heading"><h2>服务列表</h2><span>共 {query.data.total} 项</span></div><div className="eco-table-wrap"><table className="eco-table"><thead><tr><th>服务名称</th><th>服务类型</th><th>服务进度</th><th>资金状态</th><th>费用</th><th>提交时间</th><th>操作</th></tr></thead><tbody>{query.data.requests.map(r=><tr key={r.id}><td>{r.title}<small>{r.description.slice(0,60)}</small></td><td>{categories.find(c=>c.id===r.category)?.name}</td><td><span className="eco-status">{serviceStates[r.state]}</span></td><td>{financialStates[r.financialState]??"尚未收款"}</td><td>{r.quote?yuan(r.quote.amountMinor):"待报价"}</td><td>{date(r.createdAt)}</td><td><Button variant="outline" onClick={()=>setSelected(r.id)}>查看进度</Button></td></tr>)}</tbody></table></div><EcoPagination page={page} total={query.data.total} onPage={setPage}/></Card>:<ConsoleState kind="empty" title="暂无符合条件的服务">在服务市场提交需求后，可在此跟进真实服务状态。</ConsoleState>}
  </>}
  {selected?<RequestDetail key={selected} scope={scope} id={selected} onClose={()=>{setSelected("");void query.refetch()}}/>:null}
 </ConsolePage>;
}
