/* eslint-disable @next/next/no-img-element */
"use client";
import Link from "next/link";
import {useQuery} from "@tanstack/react-query";
import {ConsolePage} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import {readApplicationOverview,type MarketScope} from "@/lib/api/supply-market";
import {MarketReadError} from "./shared";
import styles from "./application-overview.module.css";

const metrics=[{key:"eligibleProducts",label:"可申请商品"},{key:"reviewing",label:"审核中"},{key:"supplementRequired",label:"需补充资料"},{key:"approved",label:"已通过"}] as const;
const steps=["选择商品","供货信息","平台评估","确认合作"];

export function SupplyApplicationOverview({scope}:{scope:MarketScope}){
 const query=useQuery({queryKey:["supply-market",scope.userId,scope.organizationId,"application-overview"],queryFn:({signal})=>readApplicationOverview(scope,signal),retry:false});
 const data=query.error?undefined:query.data;
 return <ConsolePage className={styles.overview} title="优选申请" description="将已完成智能优化、具备稳定供货能力的自有商品提交至硕米优选。" breadcrumbs={[{label:"供应市场",href:"/workbench/supply"},{label:"优选申请"}]} actions={<>
  <Button asChild className={styles.secondaryButton}><Link href="/workbench/supply/applications/records" prefetch={false}>查看申请记录</Link></Button>
  <Button asChild className={styles.primaryButton}><Link href="/workbench/supply/applications/new" prefetch={false}>选择商品发起申请</Link></Button>
 </>}>
  <section className={styles.metrics} aria-label="申请指标" aria-busy={query.isPending}>{metrics.map(metric=><Card className={styles.metric} key={metric.key}><h2>{metric.label}</h2><p>{data?data[metric.key]:query.isPending?"读取中":"不可用"}</p></Card>)}</section>
  <div className={styles.information}>
   <Card className={styles.informationCard}><section aria-labelledby="selected-conditions"><h2 id="selected-conditions">申请条件</h2><p>仅展示符合基本申请条件的已优化自有商品</p><ul>
    <li>商品属于申请人或企业</li><li>已完成商品智能优化</li><li>具备稳定库存或生产能力</li><li>商品及相关资质符合平台要求</li>
   </ul></section></Card>
   <Card className={styles.informationCard}><section aria-labelledby="selected-process"><h2 id="selected-process">申请流程</h2><p>提交后由专员联系并完成评估</p><ol className={styles.steps}>{steps.map((step,index)=><li key={step}><span>{String(index+1).padStart(2,"0")}</span><h3>{step}</h3></li>)}</ol></section></Card>
  </div>
  <Card className={styles.service}><div><h2>服务说明</h2><p>如涉及商品资料整理、系统技术对接或上架服务，将由专员确认具体服务内容及费用。</p></div><span>提交后专员联系</span></Card>
  <Card className={styles.preview}><div className={styles.previewHeader}><h2>可申请商品</h2><p>仅显示自有商品 · 已优化</p></div>
   <table className={styles.table}><thead><tr><th scope="col">商品</th><th scope="col">所属分组</th><th scope="col">商品来源</th><th scope="col">申请状态</th></tr></thead><tbody>
    {data?.products.map(product=><tr key={product.itemId}><td><div className={styles.product}><img src={product.thumbnailUrl} alt={product.title} width={40} height={40}/><span>{product.title}</span></div></td><td data-label="所属分组">{product.groupName}</td><td data-label="商品来源">自有商品</td><td data-label="申请状态"><span className={styles.eligible}>可申请</span></td></tr>)}
   </tbody></table>
   {query.isPending?<p className={styles.state} role="status">正在核对可申请商品与申请指标</p>:query.error?<MarketReadError error={query.error} retry={()=>void query.refetch()}/>:data?.products.length===0?<p className={styles.state} role="status">暂无可申请商品</p>:null}
  </Card>
  <p className={styles.note}>供货信息和资质将在提交时核验。</p>
 </ConsolePage>;
}
