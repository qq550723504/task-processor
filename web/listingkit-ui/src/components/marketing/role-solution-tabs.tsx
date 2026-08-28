"use client";

import { useState } from "react";
import { ArrowRight, Check } from "lucide-react";

import styles from "./marketing-homepage.module.css";

type RoleSolution = {
  number: string;
  title: string;
  detail: string;
  label: string;
  headline: string;
  objective: string;
  problems: string[];
  capabilities: string[];
  outcomes: string[];
  cta: string;
};

const solutions: RoleSolution[] = [
  {
    number: "01", title: "电商创业者", detail: "从零开始建立电商业务", label: "STARTER SOLUTION",
    headline: "从零开始，建立一个可执行的跨境电商业务", objective: "用户目标：选择市场、找到商品，并用 AI 团队完成业务启动",
    problems: ["不知道选择市场和平台", "不知道销售什么产品", "缺少团队与运营经验", "创业任务缺少执行路径"],
    capabilities: ["AI市场与平台选择", "AI选品和利润测算", "创业方案与任务拆解", "AI团队协同执行"],
    outcomes: ["明确创业方向", "获得可执行商品方案", "降低团队与试错成本", "更快完成业务启动"], cta: "查看创业解决方案",
  },
  {
    number: "02", title: "电商卖家", detail: "提升店铺销量与经营效率", label: "SELLER SOLUTION",
    headline: "让 AI 接管重复工作，持续提升店铺经营效率", objective: "用户目标：提升店铺销量，并降低选品、内容、运营和广告成本",
    problems: ["选品和运营依赖人工", "商品内容制作效率低", "多店铺数据相互分散", "经营问题难以及时发现"],
    capabilities: ["AI选品与商品分析", "自动生成图片、内容与上架", "AI店铺诊断与运营优化", "广告和经营数据分析"],
    outcomes: ["自动发现商品机会", "降低内容生产成本", "持续诊断店铺问题", "提高销量与运营效率"], cta: "查看卖家解决方案",
  },
  {
    number: "03", title: "工厂与供应商", detail: "把产品连接全球销售机会", label: "SUPPLIER SOLUTION",
    headline: "把产品和生产能力转化为全球销售机会", objective: "用户目标：将工厂产品数字化，并连接跨境卖家和全球渠道",
    problems: ["产品资料不够标准", "不理解海外消费者需求", "缺少跨境销售渠道", "难以触达全球卖家"],
    capabilities: ["AI产品识别与建档", "商品图片和内容优化", "市场与竞争力分析", "接入硕米优选销售生态"],
    outcomes: ["建立数字化商品档案", "提升产品市场竞争力", "对接平台卖家和渠道", "获得全球销售机会"], cta: "查看供应商解决方案",
  },
  {
    number: "04", title: "企业客户", detail: "建立企业AI电商系统", label: "ENTERPRISE SOLUTION",
    headline: "建立可沉淀、可复制的企业 AI 电商系统", objective: "用户目标：连接团队、数据和流程，推动企业业务智能化",
    problems: ["系统和数据相互割裂", "团队经验难以沉淀", "多部门协作效率低", "缺少企业专属AI能力"],
    capabilities: ["企业专属AI智能体", "企业知识库与训练", "自动化业务工作流", "数据API与权限管理"],
    outcomes: ["沉淀企业业务知识", "提升团队协作效率", "复制成熟经营经验", "建立智能化业务系统"], cta: "查看企业解决方案",
  },
  {
    number: "05", title: "OPC电商社区", detail: "一个人加AI团队共同创业", label: "OPC COMMUNITY SOLUTION",
    headline: "一个人、一支 AI 团队、一个共同成长的创业社区", objective: "用户目标：用 AI 建立个人电商项目，并连接社区资源与收益机会",
    problems: ["一个人缺少完整能力", "创业任务难以执行", "缺少商品数据和供应链", "经验能力难以变现"],
    capabilities: ["AI创业团队配置", "创业项目与任务空间", "商品数据供应链资源", "社区协作与能力变现"],
    outcomes: ["一个人拥有完整团队", "更低成本完成试错", "获得项目和资源支持", "将经验转化为收益"], cta: "查看 OPC 社区解决方案",
  },
];

export function RoleSolutionTabs() {
  const [activeIndex, setActiveIndex] = useState(1);
  const activeSolution = solutions[activeIndex];
  const panelId = `role-solution-panel-${activeIndex}`;

  return <div className={styles.roleLayout}>
    <div className={styles.roleList} role="tablist" aria-label="角色解决方案">
      {solutions.map((solution, index) => <button
        type="button"
        role="tab"
        id={`role-solution-tab-${index}`}
        aria-controls={panelId}
        aria-selected={activeIndex === index}
        className={activeIndex === index ? styles.activeRole : ""}
        key={solution.title}
        onClick={() => setActiveIndex(index)}
      ><span className={styles.roleNumber}>{solution.number}</span><span className={styles.roleCopy}><strong>{solution.title}</strong><small>{solution.detail}</small></span><ArrowRight size={15} /></button>)}
    </div>
    <article className={styles.rolePanel} id={panelId} role="tabpanel" aria-labelledby={`role-solution-tab-${activeIndex}`} aria-label={`${activeSolution.title}解决方案`}>
      <small>{activeSolution.label}</small><h3>{activeSolution.headline}</h3><p>{activeSolution.objective}</p>
      <div className={styles.roleColumns}>
        <RoleColumn title="典型问题" items={activeSolution.problems} />
        <RoleColumn title="AI 能力组合" items={activeSolution.capabilities} />
        <RoleColumn title="最终业务结果" items={activeSolution.outcomes} />
      </div>
      <a className={styles.roleCta} href="#pricing">{activeSolution.cta} <ArrowRight size={15} /></a>
    </article>
  </div>;
}

function RoleColumn({ title, items }: { title: string; items: string[] }) {
  return <div><b>{title}</b>{items.map(item => <span key={item}><Check size={13} /> {item}</span>)}</div>;
}
