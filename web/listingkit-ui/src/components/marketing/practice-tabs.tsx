"use client";

import { KeyboardEvent, useRef, useState } from "react";

import styles from "./marketing-homepage.module.css";

type Practice = {
  number: string;
  title: string;
  detail: string;
  label: string;
  headline: string;
  description: string;
  metrics: [string, string][];
  flowDescription: string;
  steps: string[];
  teamTitle: string;
  teamCopy: string;
  valueTitle: string;
  valueCopy: string;
  quote: string;
};

const practices: Practice[] = [
  { number: "01", title: "新手创业者", detail: "1天分析1,000+商品机会", label: "NEW ENTREPRENEUR  /  1天创业验证", headline: "1天完成首个跨境电商创业方案", description: "面向零经验创业者，快速完成市场、平台、产品和供应链方向验证，形成可以立即执行的30天创业任务计划。", metrics: [["1天", "完整创业方案"], ["1,000+", "商品机会分析"], ["20个", "重点测试商品"], ["20+", "供应资源匹配"]], flowDescription: "从创业想法，到一套可执行的启动方案", steps: ["需求分析", "市场筛选", "AI选品", "利润测算", "供应链匹配", "任务生成"], teamTitle: "创业者AI顾问团队", teamCopy: "AI创业顾问 · AI选品经理 · AI数据分析师\nAI供应链经理 · AI商品经理 · AI任务助手", valueTitle: "1天形成创业决策依据", valueCopy: "完成目标市场初步验证\n确定3个重点测试商品\n生成30天创业执行计划", quote: "让第一次创业，也能从清晰的数据和任务开始。" },
  { number: "02", title: "跨境电商卖家", detail: "万级Listing智能运营", label: "CROSS-BORDER SELLER  /  店铺增长", headline: "AI诊断店铺，找到被忽略的增长机会", description: "针对进入增长瓶颈的跨境店铺，分析商品、竞品、评价和广告数据，并直接生成内容优化与运营任务。", metrics: [["10,000+", "Listing批量诊断"], ["50,000+", "竞品数据分析"], ["10,000+", "营销内容生成"], ["80%", "运营效率提升"]], flowDescription: "从店铺问题发现，到持续运营优化", steps: ["店铺诊断", "竞品分析", "评价洞察", "内容优化", "广告优化", "数据跟踪"], teamTitle: "卖家AI运营团队", teamCopy: "AI运营经理 · AI数据分析师 · AI设计师\nAI广告经理 · AI客服助手 · AI任务助手", valueTitle: "把诊断结果转化为执行动作", valueCopy: "生成重点商品优化清单\n自动生成产品图片与内容\n持续跟踪运营数据变化", quote: "AI不只发现问题，还参与完成具体的运营工作。" },
  { number: "03", title: "工厂与供应商", detail: "千级产品全球化", label: "FACTORY GLOBALIZATION  /  产品出海", headline: "把传统工厂产品转化为全球可销售商品", description: "帮助工厂将现有产品数字化，识别适合出海的商品和目标市场，生成多语言商品内容并接入货盘销售生态。", metrics: [["1,000+", "产品快速数字化"], ["200+", "出海商品筛选"], ["10+", "全球市场覆盖"], ["2,000+", "渠道机会匹配"]], flowDescription: "从工厂产品资料，到全球商品资产", steps: ["资料上传", "产品识别", "市场匹配", "档案生成", "内容制作", "货盘入驻"], teamTitle: "工厂AI出海团队", teamCopy: "AI商品经理 · AI数据分析师 · AI供应链经理\nAI设计师 · AI市场顾问 · AI渠道助手", valueTitle: "让产品被全球卖家发现", valueCopy: "建立标准化商品档案\n生成多语言销售内容\n连接货盘与跨境销售渠道", quote: "工厂无需先组建海外团队，也能启动产品全球化。" },
  { number: "04", title: "企业电商团队", detail: "300+店铺智能协同", label: "ENTERPRISE AI COMMERCE  /  企业协同", headline: "用AI重构多店铺团队的运营协作", description: "连接企业知识、店铺数据和标准流程，为多平台电商团队配置专属AI员工，降低重复工作和跨部门沟通成本。", metrics: [["300+", "多平台店铺接入"], ["1,000万+", "企业知识沉淀"], ["5,000+", "自动化工作流"], ["90%", "重复任务自动化"]], flowDescription: "从分散经验，到可复制的企业AI能力", steps: ["知识接入", "店铺连接", "流程梳理", "AI员工配置", "任务协同", "经营复盘"], teamTitle: "企业专属AI团队", teamCopy: "AI运营经理 · AI数据分析师 · AI客服\nAI流程助手 · AI知识管理员 · AI管理顾问", valueTitle: "把团队经验沉淀为组织能力", valueCopy: "统一多店铺经营数据\n自动执行标准业务流程\n持续沉淀企业知识资产", quote: "让AI成为企业团队持续工作的业务协作者。" },
  { number: "05", title: "OPC电商社区", detail: "200+创业者共享AI团队", label: "OPC COMMERCE COMMUNITY  /  江西新余", headline: "一个AI智能体团队，服务200多位电商创业者", description: "为社区入驻创业者提供创业指导、AI选品、供应链匹配、商品内容与店铺运营支持，让共享空间升级为可持续服务的AI电商创业基础设施。", metrics: [["200+", "入驻创业者"], ["3,000+", "每月AI任务"], ["50,000+", "商品上线能力"], ["600万+", "月GMV目标"]], flowDescription: "从创业者提出目标，到持续跟踪经营结果", steps: ["需求识别", "创业方案", "AI选品", "供应链匹配", "商品上线", "运营跟踪"], teamTitle: "社区AI智能体团队", teamCopy: "AI创业顾问 · AI选品经理 · AI数据分析师\nAI供应链经理 · AI设计师 · AI运营经理\nAI社区助手", valueTitle: "从共享空间到AI创业基础设施", valueCopy: "统一服务200+创业项目\n沉淀可复用的创业知识和流程\n让每位创业者拥有一支AI电商团队", quote: "AI能力的价值，不是回答了多少问题，而是完成了多少业务。" },
];

export function PracticeTabs() {
  const [activeIndex, setActiveIndex] = useState(4);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const activePractice = practices[activeIndex];
  const panelId = "practice-panel";

  function selectTab(index: number) {
    setActiveIndex(index);
    tabRefs.current[index]?.focus();
  }

  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let nextIndex: number | undefined;
    if (event.key === "ArrowRight") nextIndex = (index + 1) % practices.length;
    if (event.key === "ArrowLeft") nextIndex = (index - 1 + practices.length) % practices.length;
    if (event.key === "Home") nextIndex = 0;
    if (event.key === "End") nextIndex = practices.length - 1;
    if (nextIndex === undefined) return;
    event.preventDefault();
    selectTab(nextIndex);
  }

  return <><div className={styles.practiceLayout}>
    <div className={styles.scenarioList} role="tablist" aria-label="实践场景">
      <h3>选择实践场景</h3><p>查看不同用户如何使用AI完成业务</p>
      {practices.map((practice, index) => <button type="button" role="tab" id={`practice-tab-${index}`} aria-controls={panelId} aria-selected={activeIndex === index} tabIndex={activeIndex === index ? 0 : -1} className={activeIndex === index ? styles.activeScenario : ""} key={practice.title} ref={(element) => { tabRefs.current[index] = element; }} onClick={() => setActiveIndex(index)} onKeyDown={(event) => handleTabKeyDown(event, index)}><span>{practice.number}</span><span><b>{practice.title}</b><small>{practice.detail}</small></span></button>)}
    </div>
    <article className={styles.practiceCase} id={panelId} role="tabpanel" aria-labelledby={`practice-tab-${activeIndex}`}>
      <small>{activePractice.label}</small><h3>{activePractice.headline}</h3><p>{activePractice.description}</p>
      <div className={styles.metrics}>{activePractice.metrics.map(([value, label]) => <span key={label}><strong>{value}</strong>{label}</span>)}</div>
      <h4>AI服务执行链路</h4><p className={styles.practiceFlowDescription}>{activePractice.flowDescription}</p>
      <div className={styles.executionFlow}>{activePractice.steps.map((step, index) => <span key={step}><i>{index + 1}</i>{step}</span>)}</div>
      <div className={styles.caseFooter}><p><strong>{activePractice.teamTitle}</strong>{activePractice.teamCopy}</p><p><strong>{activePractice.valueTitle}</strong>{activePractice.valueCopy}</p></div>
    </article>
  </div><blockquote className={styles.practiceQuote}>{activePractice.quote}</blockquote></>;
}
