import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight, BarChart3, Bot, Boxes, BrainCircuit, BriefcaseBusiness, Building2,
  Check, CircleDollarSign, Database, Factory, Globe2, Megaphone,
  Network, PackageSearch, Palette, Radar, ShieldCheck, ShoppingBag, Sparkles,
  Store, Truck, Users, Zap,
} from "lucide-react";

import styles from "./marketing-homepage.module.css";
import { HeroMotionLayers } from "./hero-motion";
import { ContactPanel } from "./contact-panel";
import { PracticeTabs } from "./practice-tabs";
import { RoleSolutionTabs } from "./role-solution-tabs";

const ASSET = "/sumi";
const LOGIN_HREF = "/login?returnTo=%2Flisting-kits%2Fhome";

const navItems = [["首页", "#home"], ["电商智能体", "#agents"], ["供应链与数据", "#supply-chain"], ["解决方案", "#solutions"], ["服务生态", "#services"], ["应用实践", "#practices"], ["价格与服务", "#pricing"]] as const;
const industrySteps = ["市场研究", "寻找商品", "对接供应链", "制作内容", "运营店铺", "分析增长"];
const painPoints = [["人力成本高", "大量时间消耗在重复调研、整理和执行"], ["经验难复制", "增长依赖少数人的经验和临场判断"], ["数据相互割裂", "商品、店铺、供应链数据散落在不同系统"], ["业务响应缓慢", "传统协作链路跟不上市场机会"]];
const agents = [
  [PackageSearch, "AI选品经理", "发现市场机会，找到值得销售的商品。", "市场扫描 · 潜力商品 · 竞争与利润评估", "寻找适合美国市场的宠物用品。", "blue"],
  [Store, "AI运营经理", "诊断店铺问题，持续提升销售表现。", "店铺诊断 · Listing优化 · 增长规划", "分析店铺最近销量下降的原因。", "violet"],
  [Palette, "AI设计师", "生成商品视觉和营销内容。", "商品主图 · 场景图 · 详情页与广告素材", "为户外水杯设计Amazon商品图片。", "cyan"],
  [BarChart3, "AI数据分析师", "从复杂数据中发现市场规律。", "趋势分析 · 商品数据 · 竞品与消费洞察", "分析德国小家电市场增长趋势。", "blue"],
  [Truck, "AI供应链经理", "连接商品、供应商与销售机会。", "资源匹配 · 供应商比较 · 成本利润优化", "寻找供应商并比较产品报价。", "green"],
  [Megaphone, "AI广告经理", "制定广告策略，提高投放回报。", "广告分析 · 关键词 · 预算与效果优化", "优化Amazon广告投放方案。", "amber"],
] as const;
const processSteps = [["01", "市场洞察", "国家与行业趋势", "市场机会报告"], ["02", "AI选品", "潜力商品与利润评估", "选品机会清单"], ["03", "商品开发", "需求提取与卖点规划", "商品开发方案"], ["04", "供应链匹配", "货盘搜索与风险评估", "供应链解决方案"], ["05", "店铺运营", "内容上架与经营诊断", "店铺增长方案"], ["06", "数据增长", "销售分析与策略优化", "持续增长建议"]];
const supplySources = [["硕米自营", "自有工厂与自营产品，供应稳定，快速对接销售。"], ["硕米优选", "平台筛选并经AI评估的优质商品资源。"], ["全球货盘", "连接1688及第三方货盘，整合报价与起订量。"], ["用户供应链", "上传产品、供应商与工厂，统一数字化管理。"]];
const supplyOutputs = [["数字化商品档案", "自动建立标准化产品资料与供应记录。"], ["优质供应链", "完成供应评分、成本利润与风险评估。"], ["硕米优选商品", "符合标准的产品可申请进入硕米优选。"], ["全球销售机会", "让商品连接平台卖家与全球销售渠道。"]];
const dataInsights = [["市场数据", "市场规模、国家区域、增长趋势与消费变化。", "市场机会判断"], ["商品数据", "商品销量、价格、品类趋势与生命周期。", "潜力商品发现"], ["竞品数据", "竞品价格排名、评价、卖点与变化监控。", "商品开发方向"], ["消费洞察", "搜索需求、用户评价、消费场景与产品痛点。", "店铺增长策略"]];
const services = [[Building2, "企业与资质", "公司注册 · 开户 · 年审"], [Store, "平台入驻", "店铺开通 · 认证 · 申诉"], [ShieldCheck, "品牌与合规", "商标 · 专利 · 产品认证"], [Truck, "履约与仓储", "头程 · 海外仓 · 尾程"], [CircleDollarSign, "财税与支付", "VAT · 报税 · 跨境收款"], [BriefcaseBusiness, "法律与保险", "合同 · 责任险 · 争议处理"]] as const;
const plans = [["01", "新手创业陪跑", "¥4,999 / 半年", "方向规划 · AI选品培训 · 店铺启动指导", "blue"], ["02", "店铺联合运营", "¥5,999 + 利润分成30%", "商品与市场分析 · 内容上架 · 运营执行", "green"], ["03", "品牌出海方案", "定制方案 · 咨询报价", "市场研究 · 产品组合 · 品牌本地化", "violet"], ["04", "功能定制开发", "按需求评估 · 咨询报价", "专属AI智能体 · 企业知识库 · 自动化工作流", "cyan"], ["05", "一人公司电商社区落地", "项目定制 · 咨询报价", "AI服务平台 · 资源接入 · 团队培训 · 运营陪跑", "green"]];

export function MarketingHomepage() {
  return <main className={styles.page}>
    <header className={styles.header}>
      <Link className={styles.brand} href="#home" aria-label="硕米智能引擎首页"><Image src={`${ASSET}/a5e50b9c-bfa0-4012-bb43-e53ee1a4ed17.png`} width={44} height={44} alt="" priority /><span>硕米智能引擎</span></Link>
      <nav className={styles.nav} aria-label="官网导航">{navItems.map(([label, href]) => <a href={href} key={href}>{label}</a>)}</nav>
      <Link className={styles.navCta} href={LOGIN_HREF}>进入硕米 <ArrowRight size={15} /></Link>
    </header>

    <section className={styles.hero} id="home">
      <Image className={styles.heroBackground} src={`${ASSET}/fd824975-1e65-4585-9ebf-212d68cb1507.png`} alt="" fill priority sizes="100vw" /><div className={styles.heroShade} />
      <HeroMotionLayers />
      <div className={styles.heroContent}><p className={styles.eyebrow}>新一代 AI 电商智能操作系统</p><h1>硕米智能引擎<br />新一代AI电商<br /><span>智能操作系统</span></h1><p className={styles.heroDescription}>连接AI智能体、全球商品数据、供应链资源与专业服务，让个人和组织拥有一支可执行、可协同、可增长的智能电商团队。</p><div className={styles.heroActions}><a className={styles.primaryButton} href="#agents">了解平台能力 <ArrowRight size={17} /></a><a className={styles.secondaryButton} href="#solutions">查看解决方案</a></div><div className={styles.capabilityStrip}><span><Bot size={15} /> AI智能体</span><span><Globe2 size={15} /> 全球数据</span><span><Boxes size={15} /> 商品与供应链</span><span><BriefcaseBusiness size={15} /> 专业服务</span></div></div>
      <CommerceNetwork />
    </section>

    <section className={`${styles.section} ${styles.industry}`}><SectionHeading title="电商正在进入AI时代" description="从市场洞察、商品开发到供应链和店铺增长，传统依赖人工串联的经营方式，正在被AI重新组织。" centered /><div className={styles.industryFlow}>{industrySteps.map((step, index) => <div className={styles.industryStep} key={step}><span>{String(index + 1).padStart(2, "0")}</span><strong>{step}</strong>{index < industrySteps.length - 1 ? <ArrowRight size={16} /> : null}</div>)}</div><div className={styles.painGrid}>{painPoints.map(([title, description]) => <article key={title}><Zap size={18} /><h3>{title}</h3><p>{description}</p></article>)}</div><p className={styles.statement}>真正需要改变的，不是增加更多工具，而是让 <strong>AI</strong> 重新组织整个电商流程。</p></section>

    <section className={`${styles.section} ${styles.teamSection}`}><SectionHeading label="AI COMMERCE TEAM" title="硕米让每个企业都拥有一支AI电商团队" description="围绕同一个业务目标，AI员工理解任务、调用数据、连接资源并协同执行，让复杂业务从一次回答变成持续完成。" centered /><div className={styles.teamStage}><Image className={styles.teamOrbit} src={`${ASSET}/94b9527e-2b4c-41a7-be99-7ac605a49da9.png`} width={760} height={760} alt="" /><div className={styles.teamCore}><BrainCircuit size={38} /><strong>AI 决策核心</strong><span>理解目标 · 拆解任务 · 调度执行</span></div><TeamNode className={styles.teamNodeOne} icon={<Radar />} text="市场与商品" /><TeamNode className={styles.teamNodeTwo} icon={<Truck />} text="供应链资源" /><TeamNode className={styles.teamNodeThree} icon={<Store />} text="店铺与增长" /><TeamNode className={styles.teamNodeFour} icon={<Database />} text="全球商业数据" /></div><div className={styles.teamResults}><span>一个业务目标</span><ArrowRight /><span>多智能体协作</span><ArrowRight /><span>可执行的业务结果</span></div></section>

    <section className={`${styles.section} ${styles.agentSection}`} id="agents"><SectionHeading label="打造适合自身业务的AI团队" title="每一个电商岗位，都可以拥有一位 AI 员工" description="从发现市场机会到店铺增长，硕米为不同业务岗位提供专业AI智能体。它们理解目标、调用数据、执行任务，并在协作中持续优化业务结果。" centered /><div className={styles.agentGrid}>{agents.map(([Icon, name, summary, skills, prompt, tone]) => <article className={`${styles.agentCard} ${styles[tone]}`} key={name}><div className={styles.cardTop}><span><Icon size={21} /></span><small>AI EMPLOYEE</small></div><h3>{name}</h3><p>{summary}</p><b>{skills}</b><blockquote>“{prompt}”</blockquote></article>)}</div><div className={styles.agentControl}><div><Users size={22} /><strong>AI 团队控制中心</strong></div><p>一个目标，多智能体协作。不是六个独立工具，而是一支协同工作的 AI 团队。</p><span>还可以根据企业资料，创建和训练专属 AI 员工。</span></div></section>

    <section className={`${styles.section} ${styles.processSection}`}><SectionHeading label="全流程 AI 电商" title="从发现机会到实现增长，全流程由 AI 协同完成" description="硕米将市场、商品、供应链、内容、店铺和数据连接成完整业务流程，让每一步都可以被理解、执行和持续优化。" centered /><div className={styles.processGrid}>{processSteps.map(([number, title, detail, output]) => <article key={number}><span>{number}</span><h3>{title}</h3><p>{detail}</p><small>输出结果</small><b>{output}</b></article>)}</div><div className={styles.processFooter}><div><Network size={24} /><p><strong>一套系统，连接完整电商业务。</strong><span>硕米智能引擎会根据任务自动调用所需能力。</span></p></div><a href="#solutions">查看解决方案 <ArrowRight size={16} /></a></div></section>

    <section className={`${styles.section} ${styles.supplySection}`} id="supply-chain"><SectionHeading label="AI 智能供应链" title="连接商品、货盘与工厂，让好产品快速进入市场" description="硕米连接自营产品、精选供应商、全球货盘和用户自有供应链，并通过AI完成产品识别、信息优化、供应评估和销售机会匹配。" centered /><div className={styles.supplyMap}><SupplyColumn items={supplySources} offset={1} /><div className={styles.supplyCore}><Image src={`${ASSET}/c466e231-1231-4986-9cc9-b524c43a8cfc.svg`} fill sizes="440px" alt="" /><BrainCircuit size={36} /><strong>硕米 AI<br />供应链引擎</strong><span>识别 · 优化<br />评估 · 匹配</span></div><SupplyColumn items={supplyOutputs} offset={5} /></div><div className={styles.onboardingBar}><strong>你的产品，也可以进入硕米销售生态</strong>{["上传产品资料", "AI识别产品", "生成产品档案", "供应链评分", "获得销售机会"].map((step, index) => <span key={step}>{index + 1}. {step}</span>)}</div></section>

    <section className={`${styles.section} ${styles.dataSection}`}><SectionHeading label="全球商业数据网络" title="用数据理解市场，发现下一个增长机会" description="硕米连接全球市场、商品、竞品和消费数据，让AI的每一次分析、选品与经营决策都有数据依据。" centered /><div className={styles.dataEngine}><Image src={`${ASSET}/031ee5d5-089b-429f-bc11-f639f3cb9bf1.svg`} fill sizes="920px" alt="" /><div className={styles.dataCore}><Globe2 size={43} /><strong>硕米全球数据引擎</strong><span>全球数据 → 智能决策</span><small>公开、授权及用户接入数据</small></div><div className={`${styles.platformBubble} ${styles.bubbleOne}`}>Amazon · eBay</div><div className={`${styles.platformBubble} ${styles.bubbleTwo}`}>SHEIN · TEMU</div><div className={`${styles.platformBubble} ${styles.bubbleThree}`}>TikTok · 社交内容</div><div className={`${styles.platformBubble} ${styles.bubbleFour}`}>淘宝 · 1688 · 货源</div></div><div className={styles.insightGrid}>{dataInsights.map(([title, description, result]) => <article key={title}><Database size={18} /><h3>{title}</h3><p>{description}</p><span><ArrowRight size={14} /> {result}</span></article>)}</div><div className={styles.dataQuery}><p>“帮我分析美国宠物用品市场，并寻找国内供应资源。”</p><div><span>市场趋势</span><span>内容热度</span><span>商品价格</span><span>供应资源</span><strong>AI 商业建议</strong></div></div></section>

    <section className={`${styles.section} ${styles.solutionSection}`} id="solutions"><SectionHeading label="角色解决方案" title="不同的业务起点，同一套 AI 增长能力" description="硕米根据不同用户的资源、经验和业务目标，组合AI智能体、数据、供应链和工具能力，提供针对性的业务路径。" centered /><RoleSolutionTabs /><div className={styles.opcBanner}><div><Users size={27} /><span><strong>一个人、一支 AI 团队、一个共同成长的电商创业社区</strong><small>创建个人项目，配置AI员工，接入商品、数据和供应链。</small></span></div><a href="#practices">查看 OPC 创业案例 <ArrowRight size={15} /></a></div></section>

    <section className={`${styles.section} ${styles.serviceSection}`} id="services"><SectionHeading label="GLOBAL PROFESSIONAL SERVICES / 全球专业服务生态" title="AI完成线上业务，专业机构负责关键落地" description="硕米连接企业设立、平台入驻、品牌合规、跨境履约、财税支付和法律保险等专业资源，让AI方案真正转化为可执行的全球业务。" /><div className={styles.servicesLayout}><div className={styles.serviceNetwork}><div className={styles.serviceCore}><Bot size={31} /><strong>AI 方案协调中心</strong><span>需求识别 · 服务匹配</span></div>{services.map(([Icon, title, detail]) => <article key={title}><Icon size={18} /><div><h3>{title}</h3><p>{detail}</p></div></article>)}</div><div className={styles.serviceDetail}><small>BRAND & COMPLIANCE</small><h3>品牌、知识产权与合规</h3><p>由专业机构完成申请、认证、备案与争议处理，帮助商品满足目标市场和销售平台要求。</p><div>{["商标注册", "品牌备案", "专利申请", "产品认证", "EPR合规", "责任保险", "侵权申诉", "合规咨询"].map(item => <span key={item}><Check size={13} /> {item}</span>)}</div><ol><li>提交需求</li><li>匹配机构</li><li>材料办理</li><li>进度交付</li></ol></div></div><div className={styles.serviceMatching}><Sparkles size={22} /><p><strong>专业服务智能匹配</strong><span>经营目标：将儿童玩具销售到德国 Amazon</span></p><div>德国商标 → 产品认证 → EPR合规 → 平台入驻 → VAT注册 → 欧洲海外仓</div></div></section>

    <section className={`${styles.section} ${styles.practiceSection}`} id="practices"><SectionHeading label="AI COMMERCE PRACTICES / AI电商应用实践" title="从一个业务目标，到一套可以执行的结果" description="硕米智能引擎组织AI员工、商品数据、供应链资源和专业服务，帮助不同类型的用户完成完整电商任务。" /><PracticeTabs /></section>

    <section className={`${styles.section} ${styles.pricingSection}`} id="pricing"><SectionHeading label="价格与服务方案" title="简单透明，只为实际使用付费" description="注册即可免费体验。店铺按月接入，人工智能按实际算力用量消耗计费；需要业务落地支持时，可选择对应的专项服务。" centered /><div className={styles.trialBanner}><span>新用户专享</span><strong>注册即享15天免费体验</strong><p>免费绑定1个店铺 · 赠送¥5 AI算力券 · 体验店铺接入与AI能力 · 试用结束不自动续费</p><Link href={LOGIN_HREF}>免费开始体验 <ArrowRight size={15} /></Link></div><div className={styles.basePricing}><article><Store size={25} /><small>店铺基础接入</small><h3><strong>¥99</strong> / 店铺 / 月</h3><p>数据安全接入 · 商品与订单同步 · 经营看板<br />多店铺统一管理 · AI智能体调用入口</p></article><article><Zap size={25} /><small>人工智能算力</small><h3><strong>按算力用量</strong> 实际消耗计费</h3><p>不调用AI不产生费用 · 执行前显示预估消耗<br />提供用量明细 · 支持预算设置 · 余额不足自动暂停</p></article></div><div className={styles.professionalHeader}><div><span>专项落地服务</span><h3>从使用人工智能工具，到完成业务落地</h3></div><p>根据不同业务阶段，选择创业指导、联合运营、品牌出海、功能开发或社区建设服务。</p></div><div className={styles.planGrid}>{plans.map(([number, title, price, detail, tone]) => <article className={styles[tone]} key={title}><span>{number}</span><h3>{title}</h3><b>{price}</b><p>{detail}</p></article>)}</div><p className={styles.pricingNote}>统一收费说明：15天体验仅限1个店铺 · AI按实际算力用量计费 · 联合运营需通过项目评估 · 不承诺固定销售额或固定利润</p></section>

    <footer className={styles.footer}><div className={styles.footerBrand}><Image src={`${ASSET}/c3ea4c6f-992f-4c3d-ae2e-c14d4ec9735a.png`} width={38} height={38} alt="" /><div><strong>硕米智能引擎</strong><span>新一代人工智能电商操作系统</span></div></div><div className={styles.footerLinks}><a href="#agents">产品能力</a><a href="#solutions">解决方案</a><a href="#services">服务支持</a><a href="#practices">应用实践</a><a href="#pricing">价格方案</a></div><div className={styles.copyright}>© 2026 硕米智能引擎　保留所有权利 <span>隐私政策　用户协议　算力计费说明　服务协议</span></div></footer>
    <ContactPanel />
  </main>;
}

function SectionHeading({ label, title, description, centered = false }: { label?: string; title: string; description: string; centered?: boolean }) { return <div className={`${styles.sectionHeading} ${centered ? styles.centered : ""}`}>{label ? <p>{label}</p> : null}<h2>{title}</h2><span>{description}</span></div>; }
function CommerceNetwork() { return <div className={styles.commerceNetwork} aria-label="硕米智能商业网络"><Image className={styles.networkRing} src={`${ASSET}/430cfb70-82cd-4b7d-bcf9-754e700732f5.svg`} width={588} height={328} alt="" /><Image className={styles.networkRingInner} src={`${ASSET}/24ad7c83-a9a6-47ef-aff8-b2b9e3dfb4c0.svg`} width={420} height={226} alt="" /><div className={styles.networkCore}><small>硕米智能引擎</small><strong>全球商业智能网络</strong><span>● 四大能力协同运行</span></div><NetworkNode className={styles.networkNodeOne} number="01" title="AI智能体" detail="驱动智能决策" icon={<Bot />} /><NetworkNode className={styles.networkNodeTwo} number="02" title="全球商品数据" detail="汇集全球商品信息" icon={<Database />} /><NetworkNode className={styles.networkNodeThree} number="03" title="供应链货盘" detail="链接优质供应资源" icon={<Factory />} /><NetworkNode className={styles.networkNodeFour} number="04" title="生态服务" detail="整合商业服务能力" icon={<ShoppingBag />} /></div>; }
function NetworkNode({ className, number, title, detail, icon }: { className: string; number: string; title: string; detail: string; icon: React.ReactNode }) { return <div className={`${styles.networkNode} ${className}`}><span>{number}</span><div><b>{title}</b><small>{detail}</small></div>{icon}</div>; }
function TeamNode({ className, icon, text, network = false }: { className: string; icon: React.ReactNode; text: string; network?: boolean }) { return <div className={`${network ? styles.networkNode : styles.teamNode} ${className}`}>{icon}<b>{text}</b></div>; }
function SupplyColumn({ items, offset }: { items: string[][]; offset: number }) { return <div className={styles.supplyColumn}>{items.map(([title, description], index) => <article key={title}><span>0{index + offset}</span><div><h3>{title}</h3><p>{description}</p></div></article>)}</div>; }
