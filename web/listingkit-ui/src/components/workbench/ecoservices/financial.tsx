"use client";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {Button} from "@/components/ui/button";
import {ecoRequest,ecoFinancialSchema,type EcoScope} from "@/lib/api/ecoservices";
import {ReadFailure,yuan,type useEcoCommands} from "./shared";

export function FinancialFacts({scope,id,commands}:{scope:EcoScope;id:string;commands:ReturnType<typeof useEcoCommands>}){
 const [billDate,setBillDate]=useState("");
 const query=useQuery({queryKey:["ecoservices",scope.userId,scope.organizationId,"financial",id],queryFn:({signal})=>ecoRequest(scope,"requests/"+id+"/financial",ecoFinancialSchema,{signal},true)});
 const f=query.data;
 return <section><h3>渠道资金事实</h3>{query.isPending?<p>正在读取原资金记录…</p>:query.error?<ReadFailure error={query.error} retry={()=>void query.refetch()}/>:f?<><dl><dt>原订单金额</dt><dd>{yuan(f.grossMinor)}</dd>{f.channelAmounts?<><dt>实际现金付款 / 应结收入</dt><dd>{yuan(f.channelAmounts.payerMinor)} / {yuan(f.channelAmounts.settlementMinor)}</dd><dt>已退现金 / 已退应结收入</dt><dd>{yuan(f.channelAmounts.payerRefundedMinor)} / {yuan(f.channelAmounts.settlementRefundedMinor)}</dd></>:null}<dt>已确认退款</dt><dd>{yuan(f.refundedMinor)}</dd><dt>已确认拒付</dt><dd>{yuan(f.chargedBackMinor)}</dd><dt>剩余平台佣金 / 服务商份额</dt><dd>{yuan(f.platformMinor)} / {yuan(f.providerMinor)}</dd><dt>已分佣金 / 已回退佣金</dt><dd>{yuan(f.sharedMinor)} / {yuan(f.returnedMinor)}</dd><dt>已确认渠道释放</dt><dd>{yuan(f.releasedMinor)}</dd><dt>已核实渠道费用</dt><dd>{f.channelFeeObserved?yuan(f.channelFeeMinor):"尚无账单事实"}</dd></dl>{f.reconciliationReason?<p>资金核实暂停原因：{f.reconciliationReason}</p>:null}<p>费用仅累计已核实的渠道流水，后续费用及退费以账单为准。无账单不计作零费用，不据此宣称最终净收益。</p></>:null}
 <form onSubmit={e=>{e.preventDefault();void commands.execute({path:"requests/"+id+"/fees",key:crypto.randomUUID(),body:JSON.stringify({date:billDate}),output:"financial",admin:true}).then(()=>query.refetch()).catch(()=>undefined)}}><fieldset disabled={commands.locked}><label>渠道账单日期<input required type="date" value={billDate} onChange={e=>setBillDate(e.target.value)}/></label><p>由原平台商户读取手续费账户账单，仅记录本订单关联的实际流水。账单通常在次日生成，支持近三个月。</p><Button type="submit" variant="outline">核实该日手续费流水</Button></fieldset></form>
 </section>;
}
