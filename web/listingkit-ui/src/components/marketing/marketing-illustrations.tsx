import Image from "next/image";
import { MotionLayer } from "./marketing-motion";
import styles from "./marketing-illustrations.module.css";

const TEAM_ROLES = ["AI选品经理", "AI运营经理", "AI设计师", "AI数据分析师", "AI供应链经理"];
export function MarketingTeam() {
  return <><div className={styles.teamLayout}>
    <aside className={styles.teamPanel}><small>用户想做什么</small><h3>一句话提出业务目标</h3><ul>{["寻找潜力商品","分析目标市场","优化商品表现","为工厂寻找销路"].map(label => <li key={label}><div className={styles.panelDot}><Image src="/sumi/figma-home-team-imgEllipse.svg" alt="" width={6} height={6} unoptimized /></div>{label}</li>)}</ul></aside>
    <div className={styles.teamCanvas} role="group" aria-label="AI电商团队能力示意">
      <MotionLayer className={styles.teamAmbient} nodeId="63:3"><Image src="/sumi/figma-home-team-img.svg" alt="" width={760} height={760} unoptimized /></MotionLayer>
<MotionLayer className={styles.teamOrbitOuter} nodeId="63:7"><Image src="/sumi/94b9527e-2b4c-41a7-be99-7ac605a49da9.png" alt="" width={500} height={500} unoptimized /></MotionLayer>
<div className={styles.teamOrbitMiddle}><MotionLayer nodeId="63:8"><Image src="/sumi/6fbc01df-5d99-4396-bc1f-a3f04127d377.png" alt="" width={390} height={390} unoptimized /></MotionLayer></div>
<div className={styles.teamOrbitInner}><MotionLayer nodeId="63:9"><Image src="/sumi/d1442f3f-5547-4df8-acef-34949704231c.png" alt="" width={292} height={292} unoptimized /></MotionLayer></div>
<MotionLayer className={styles.teamGlow} nodeId="63:10"><Image src="/sumi/figma-home-team-imgAi3.svg" alt="" width={326} height={326} unoptimized /></MotionLayer>
<MotionLayer className={styles.teamCoreArt} nodeId="63:11"><Image src="/sumi/figma-home-team-imgAi4.svg" alt="" width={184} height={184} unoptimized /></MotionLayer><MotionLayer className={styles.particle0} nodeId="358:303"><Image src="/sumi/figma-home-team-img01.svg" alt="" width={22} height={22} unoptimized /></MotionLayer><MotionLayer className={styles.particle1} nodeId="358:304"><Image src="/sumi/figma-home-team-img02.svg" alt="" width={24} height={24} unoptimized /></MotionLayer><MotionLayer className={styles.particle2} nodeId="358:305"><Image src="/sumi/figma-home-team-img03.svg" alt="" width={21} height={21} unoptimized /></MotionLayer><MotionLayer className={styles.particle3} nodeId="358:306"><Image src="/sumi/figma-home-team-img04.svg" alt="" width={23} height={23} unoptimized /></MotionLayer><MotionLayer className={styles.particle4} nodeId="358:307"><Image src="/sumi/figma-home-team-img05.svg" alt="" width={22} height={22} unoptimized /></MotionLayer><MotionLayer className={styles.particle5} nodeId="358:308"><Image src="/sumi/figma-home-team-img06.svg" alt="" width={21} height={21} unoptimized /></MotionLayer><MotionLayer className={styles.particle6} nodeId="358:309"><Image src="/sumi/figma-home-team-img07.svg" alt="" width={21} height={21} unoptimized /></MotionLayer><MotionLayer className={styles.particle7} nodeId="358:310"><Image src="/sumi/figma-home-team-img08.svg" alt="" width={22} height={22} unoptimized /></MotionLayer><MotionLayer className={styles.particle8} nodeId="358:311"><Image src="/sumi/figma-home-team-img09.svg" alt="" width={23} height={23} unoptimized /></MotionLayer><MotionLayer className={styles.particle9} nodeId="358:312"><Image src="/sumi/figma-home-team-img10.svg" alt="" width={21} height={21} unoptimized /></MotionLayer><MotionLayer className={styles.particle10} nodeId="358:313"><Image src="/sumi/figma-home-team-img11.svg" alt="" width={23} height={23} unoptimized /></MotionLayer><MotionLayer className={styles.particle11} nodeId="358:314"><Image src="/sumi/figma-home-team-img12.svg" alt="" width={22} height={22} unoptimized /></MotionLayer>
      <div className={styles.teamCoreText}><strong>硕米 AI<br />决策中枢</strong><span>理解 · 拆解 · 调度 · 优化</span></div>
      <ul className={styles.teamRoles}>{TEAM_ROLES.map((label,index) => <li key={label} className={styles['role'+index]}><div className={styles.roleDot}><Image src="/sumi/figma-home-team-img1.svg" alt="" width={26} height={26} unoptimized /></div>{label}</li>)}</ul>
    </div>
    <aside className={styles.teamPanel}><small>智能生态系统</small><h3>商业能力持续接入</h3><ul>{["全球数据","商品与货盘","供应链资源","工具与生态服务"].map(label => <li key={label}><div className={styles.panelDot}><Image src="/sumi/figma-home-team-imgEllipse.svg" alt="" width={6} height={6} unoptimized /></div>{label}</li>)}</ul></aside>
  </div><ul className={styles.teamResults}>{["更快发现市场机会","更低的经营成本","更高的运营效率","可持续复制的增长"].map(label => <li key={label}><div className={styles.resultDot}><Image src="/sumi/figma-home-team-imgEllipse1.svg" alt="" width={24} height={24} unoptimized /></div>{label}</li>)}</ul>
  <div className={styles.teamBottom}><p>你只需要提出目标，剩下的交给你的 AI 电商团队。</p><a href="#agents">探索 AI 智能体 →</a></div></>;
}

export function BrandClosing() {
  return <section className={styles.closing} aria-labelledby="brand-closing-title" data-node-id="234:203">
    <div className={styles.closingCopy}><small>硕米智能引擎</small><h2 id="brand-closing-title">让每个人，都拥有一支智能电商团队</h2><p>连接人工智能员工、全球商品数据、供应链资源和专业服务，把复杂的电商业务转化为可以理解、可以执行、可以持续优化的任务。</p><ul>{[["降低门槛","让新手更快开始"],["提升效率","让卖家规模化经营"],["连接全球","让好产品走向市场"]].map(([title,detail]) => <li key={title}><strong>{title}</strong><span>{detail}</span></li>)}</ul></div>
    <div className={styles.closingVisual}><div className={styles.closingGlow}><Image src="/sumi/figma-home-footer-img.svg" alt="" width={420} height={420} unoptimized /></div>
<MotionLayer className={styles.closingAmbient} nodeId="356:303"><Image src="/sumi/figma-home-footer-img1.svg" alt="" width={360} height={180} unoptimized /></MotionLayer>
<MotionLayer className={styles.closingFlow0} nodeId="356:304"><Image src="/sumi/figma-home-footer-img01.svg" alt="" width={360} height={76} unoptimized /></MotionLayer>
<MotionLayer className={styles.closingFlow1} nodeId="356:306"><Image src="/sumi/figma-home-footer-img02.svg" alt="" width={360} height={76} unoptimized /></MotionLayer>
<MotionLayer className={styles.closingFlow2} nodeId="356:308"><Image src="/sumi/figma-home-footer-img03.svg" alt="" width={360} height={76} unoptimized /></MotionLayer>
<MotionLayer className={styles.closingDot0} nodeId="356:310"><Image src="/sumi/figma-home-footer-img2.svg" alt="" width={33} height={33} unoptimized /></MotionLayer>
<MotionLayer className={styles.closingDot1} nodeId="356:311"><Image src="/sumi/figma-home-footer-img3.svg" alt="" width={36} height={36} unoptimized /></MotionLayer>
<MotionLayer className={styles.closingDot2} nodeId="356:312"><Image src="/sumi/figma-home-footer-img4.svg" alt="" width={32} height={32} unoptimized /></MotionLayer><blockquote><small>人工智能能力的价值，</small><p>不在于回答多少问题，</p><strong>而在于创造多少的价值</strong></blockquote></div>
  </section>;
}
