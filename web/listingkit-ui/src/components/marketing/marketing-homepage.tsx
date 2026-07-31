import Image from "next/image";
import Link from "next/link";
import { ArrowRight, BadgeCheck, Bot, Boxes, CalendarCheck2, Check, CircleCheck, Images, Layers3, Paintbrush2, ScanSearch, ShieldCheck, Sparkles, TrendingDown } from "lucide-react";

import styles from "./marketing-homepage.module.css";
import { DemoRequestForm } from "./demo-request-form";

const capabilities = [
  { icon: Layers3, title: "一份资料，多平台上架", description: "商品标题、属性、变体、价格与素材只需整理一次，就能生成适配不同销售渠道的独立资料包。" },
  { icon: Bot, title: "AI 做完繁琐工作", description: "自动提炼商品事实、生成文案、补全属性、整理图片与适配规则，省掉反复复制、改写和录入。" },
  { icon: BadgeCheck, title: "团队只管审核与决策", description: "AI 先把每个平台版本准备好；运营只需查看差异、处理阻断项并确认发布，不再从零制作。" },
];

const operations = [
  { icon: CalendarCheck2, status: "已上线", title: "自动报名活动", description: "根据店铺与商品条件自动筛选机会、编排报名任务，让活动运营不再依赖重复操作。" },
  { icon: TrendingDown, status: "规划中", title: "低流量商品自动下架", description: "持续识别低效商品，结合可配置策略自动治理库存与在售结构。" },
  { icon: Paintbrush2, status: "已支持", title: "POD 商品生产", description: "从设计、变体到 mockup 素材集中管理，让按需定制商品更快进入上架流程。" },
  { icon: Images, status: "已支持", title: "图片裂变与素材增强", description: "围绕主图、白底图和场景图组织素材，并生成更适配渠道的商品视觉表达。" },
];

const targetPlatforms = [
  { name: "SHEIN", detail: "商品资料、审核与发布流程" },
  { name: "TEMU", detail: "适配渠道要求的资料版本" },
  { name: "Amazon", detail: "结构化商品信息与 Listing 输出" },
  { name: "TikTok Shop", detail: "面向内容电商的商品上架版本" },
];

const controlPoints = [
  { icon: ScanSearch, title: "阻断项直达修复", description: "类目、属性、图片、价格和 SKU 问题被清晰标出，运营可以直接进入对应区域处理。" },
  { icon: BadgeCheck, title: "审核门槛前置", description: "在保存草稿或提交前完成资料检查与最终确认，避免不完整资料流入渠道。" },
  { icon: ShieldCheck, title: "失败可追踪、可恢复", description: "对生成、预览、上传与发布失败保留明确的状态与下一步，减少查日志和反复沟通。" },
];

const workflow = [
  ["01", "录入一次", "导入 1688、POD 或已有商品资料"],
  ["02", "AI 拆解", "提炼商品事实、变体、卖点与素材"],
  ["03", "多端生成", "输出每个渠道需要的差异化资料包"],
  ["04", "审核分发", "确认后保存草稿或提交至目标平台"],
];

export function MarketingHomepage() {
  return (
    <main className={styles.page}>
      <div className={styles.ambient} aria-hidden="true" />
      <header className={styles.header}>
        <Link className={styles.brand} href="/" aria-label="ListingKit 首页"><span className={styles.brandMark}><Boxes size={18} /></span>Listing<span>Kit</span></Link>
        <nav className={styles.nav} aria-label="官网导航"><a href="#platforms">平台</a><a href="#capabilities">能力</a><a href="#operations">运营</a><a href="#workflow">流程</a></nav>
        <a className={styles.navCta} href="#contact">预约演示 <ArrowRight size={15} /></a>
      </header>

      <section className={styles.hero}>
        <div className={styles.heroCopy}>
          <p className={styles.eyebrow}><Sparkles size={15} /> AI 驱动的跨境多平台运营中枢</p>
          <h1>一份资料，<br /><em>多平台增长。</em></h1>
          <p className={styles.lede}>商品资料只需录入一次。ListingKit 让 AI 自动完成提炼、改写、配图与渠道适配，为每个平台生成可审核的上架版本；团队只负责审核与决策。</p>
          <div className={styles.heroActions}><Link className={styles.primaryCta} href="/listing-kits/home">进入工作台 <ArrowRight size={17} /></Link><a className={styles.secondaryCta} href="#contact">预约演示 <span>↓</span></a></div>
          <div className={styles.trustRow}><span><Check size={15} /> 一次录入，多端复用</span><span><Check size={15} /> AI 先做，人来审核</span><span><Check size={15} /> 每个平台独立适配</span></div>
          <div className={styles.platformStrip} aria-label="支持的目标渠道"><span>一次输入，分别输出</span><b>SHEIN</b><b>TEMU</b><b>Amazon</b><b>TikTok Shop</b></div>
        </div>

        <div className={styles.heroVisual} aria-label="ListingKit 商品资料流转示意">
          <div className={styles.grid} />
          <div className={styles.sourceStack}><ProductCard name="休闲双肩包" tone="sand" /><ProductCard name="运动水壶" tone="blue" /><ProductCard name="通勤帆布鞋" tone="pink" /></div>
          <div className={styles.flowLine}><span /><span /><span /></div>
          <div className={styles.aiCore}><Sparkles size={29} /><strong>AI</strong><small>CONTENT ENGINE</small></div>
          <div className={styles.outputStack}><ListingCard label="SHEIN 上架资料" status="待审核" /><ListingCard label="TikTok Shop 版本" status="待审核" /></div>
          <div className={styles.reviewPanel}><div><CircleCheck size={18} /><b>Ready for review</b></div><p><span /> 标题与属性已匹配</p><p><span /> 素材规范已检查</p><p><span /> 上架资料已就绪</p></div>
        </div>
      </section>

      <section className={styles.platformCoverage} id="platforms">
        <div className={styles.coverageCopy}><p className={styles.eyebrow}>渠道不是复制粘贴，是独立适配</p><h2>同一件商品，<br />准备好每一种卖法。</h2><p>统一保存商品事实，再由 AI 依据目标渠道的资料结构、表达方式和审核要求，生成每个平台各自可用的版本。</p></div>
        <div className={styles.platformGrid}>{targetPlatforms.map(({ name, detail }, index) => <article className={styles.platformCard} key={name}><span>0{index + 1}</span><h3>{name}</h3><p>{detail}</p><b>目标渠道</b></article>)}</div>
      </section>

      <section className={styles.capabilities} id="capabilities">
        <div className={styles.sectionIntro}><p className={styles.eyebrow}>不再为每个平台从头做一遍</p><h2>商品资料生产，<br />从人工制作变成 AI 审核流。</h2></div>
        <div className={styles.capabilityGrid}>{capabilities.map(({ icon: Icon, title, description }, index) => <article className={styles.capability} key={title}><span className={styles.cardIndex}>0{index + 1}</span><span className={styles.iconBox}><Icon size={22} /></span><h3>{title}</h3><p>{description}</p></article>)}</div>
      </section>

      <section className={styles.showcase} aria-labelledby="showcase-title">
        <div className={styles.showcaseImage}><Image src="/hero-multichannel.png" alt="一份商品资料经过 AI 处理后生成多个渠道上架版本" width={1734} height={908} priority /></div>
        <div className={styles.showcaseCopy}><p className={styles.eyebrow}>从一份资料，到多个增长出口</p><h2 id="showcase-title">AI 负责生成，<br />你负责判断。</h2><p>同一商品不再在不同渠道重复录入、反复改写。ListingKit 先完成每个平台的资料生产，运营只在最后一步检查差异与决定是否发布。</p><a className={styles.textCta} href="#workflow">查看完整工作流 <span>↓</span></a></div>
      </section>

      <section className={styles.operations} id="operations">
        <div className={styles.operationsIntro}><p className={styles.eyebrow}>不止上架，更懂运营节奏</p><h2>把重复运营动作，<br />交给自动化。</h2><p>从活动报名到商品治理，再到 POD 与图片生产，ListingKit 正在把运营经验沉淀为可持续执行的流程。</p></div>
        <div className={styles.operationGrid}>{operations.map(({ icon: Icon, status, title, description }) => <article className={styles.operationCard} key={title}><div className={styles.operationMeta}><span className={status === "规划中" ? styles.planned : styles.live}>{status}</span><Icon size={21} /></div><h3>{title}</h3><p>{description}</p><span className={styles.operationArrow}>↗</span></article>)}</div>
      </section>

      <section className={styles.controlPoints}>
        <div className={styles.controlHeading}><p className={styles.eyebrow}>自动化不等于失控</p><h2>每一步自动化，<br />都留给人清晰的判断。</h2></div>
        <div className={styles.controlGrid}>{controlPoints.map(({ icon: Icon, title, description }) => <article key={title}><span><Icon size={20} /></span><h3>{title}</h3><p>{description}</p></article>)}</div>
      </section>

      <section className={styles.workflow} id="workflow"><div className={styles.workflowHeading}><div><p className={styles.eyebrow}>从来源商品到目标渠道</p><h2>每一步，都有明确的下一步。</h2></div></div><div className={styles.workflowSteps}>{workflow.map(([number, title, description]) => <article key={number}><span>{number}</span><h3>{title}</h3><p>{description}</p></article>)}</div></section>

      <section className={styles.security} id="security"><div className={styles.securityIcon}><ShieldCheck size={30} /></div><div><p className={styles.eyebrow}>为团队协作而生</p><h2>让资料、权限与责任边界保持清晰。</h2></div><p>从身份认证到租户隔离，ListingKit 让不同团队在同一套工作流中协作，同时确保商品资料与操作权限各归其位。</p></section>

      <section className={styles.contact} id="contact"><p className={styles.eyebrow}>预约产品演示</p><h2>让每一份商品资料，<br />成为更多渠道的增长起点。</h2><p>留下你的信息，我们会基于商品来源、运营流程和目标渠道准备演示方案。</p><DemoRequestForm /><Link className={styles.contactWorkspaceLink} href="/listing-kits/home">已有账号，直接进入工作台 <ArrowRight size={15} /></Link></section>

      <footer className={styles.footer}><Link className={styles.brand} href="/"><span className={styles.brandMark}><Boxes size={18} /></span>Listing<span>Kit</span></Link><p>让商品资料成为持续增长的基础设施。</p><a href="#contact">预约演示 <ArrowRight size={15} /></a></footer>
    </main>
  );
}

function ProductCard({ name, tone }: { name: string; tone: "sand" | "blue" | "pink" }) {
  return <article className={`${styles.productCard} ${styles[tone]}`}><span className={styles.productImage} /><b>{name}</b><small>原始商品资料</small></article>;
}

function ListingCard({ label, status }: { label: string; status: string }) {
  return <article className={styles.listingCard}><span className={styles.listingThumb} /><div><b>{label}</b><small>标题 · 属性 · 素材</small><i>{status}</i></div></article>;
}
