import Link from "next/link";
import {ConsolePage} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import styles from "./catalog-overview.module.css";

function FlowStep({number,title,detail,final=false}:{number:string;title:string;detail?:string;final?:boolean}) {
  return <li className={`${styles.step} ${final?styles.finalStep:""}`}>
    <span className={styles.number}>{number}</span>
    <div><h3>{title}</h3>{detail?<p>{detail}</p>:null}</div>
  </li>;
}

export function CatalogOverview() {
  return <ConsolePage className={styles.overview} title="货盘集成"
    description="汇集第三方货盘，为用户提供统一的商品选择与供应链沉淀入口。"
    breadcrumbs={[{label:"供应市场",href:"/workbench/supply"},{label:"货盘集成"}]}>
    <div className={styles.entries}>
      <Card className={styles.entry}>
        <span className={styles.kicker}>官方货盘选品入口</span>
        <h2>货盘选品</h2>
        <p>浏览当前可用的货盘能力，选择合适的商品加入我的供应链。</p>
        <Button asChild className={styles.entryButton}><Link href="/workbench/supply/catalogs/selection" prefetch={false}>进入货盘选品</Link></Button>
      </Card>
      <Card className={`${styles.entry} ${styles.connectionEntry}`}>
        <span className={styles.kicker}>需要的货盘尚未接入</span>
        <h2>申请对接</h2>
        <p>提交货盘资料，由硕米专员评估业务、数据与技术对接方案。</p>
        <Button asChild className={styles.entryButton}><Link href="/workbench/supply/catalogs/apply" prefetch={false}>申请对接货盘</Link></Button>
      </Card>
    </div>
    <div className={styles.processes}>
      <Card className={styles.process}>
        <section aria-labelledby="catalog-product-process">
          <h2 id="catalog-product-process">商品进入供应链流程</h2>
          <p className={styles.processDescription}>选择货盘产品或使用采集工具，再沿现有流程加入供应链、优化与发布。</p>
          <div className={styles.productFlow}>
            <ul className={styles.sources} aria-label="商品来源">
              <FlowStep number="A" title="选择货盘产品" detail="从官方货盘中选品"/>
              <FlowStep number="B" title="通过采集工具" detail="采集其他商品"/>
            </ul>
            <ol className={styles.productSteps} aria-label="商品进入供应链步骤">
              <FlowStep number="01" title="加入我的供应链" detail="统一管理商品"/>
              <FlowStep number="02" title="调用智能体或工具" detail="优化图片与资料"/>
              <FlowStep number="03" title="发布到关联店铺"/>
            </ol>
          </div>
        </section>
      </Card>
      <Card className={styles.process}>
        <section aria-labelledby="catalog-connection-process">
          <h2 id="catalog-connection-process">申请对接流程</h2>
          <p className={styles.processDescription}>需要的货盘尚未接入时，提交资料由硕米专员评估与确认对接安排。</p>
          <ol className={styles.connectionSteps} aria-label="申请对接步骤">
            <FlowStep number="01" title="提交货盘资料"/>
            <FlowStep number="02" title="硕米业务评估"/>
            <FlowStep number="03" title="数据与接口校验"/>
            <FlowStep number="04" title="完成技术对接并开放选品" final/>
          </ol>
        </section>
      </Card>
    </div>
  </ConsolePage>;
}
