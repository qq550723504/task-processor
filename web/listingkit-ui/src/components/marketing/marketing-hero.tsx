"use client";

import Image from "next/image";
import Link from "next/link";
import { ArrowRight, Menu } from "lucide-react";
import { MotionLayer } from "./marketing-motion";
import styles from "./marketing-hero.module.css";

const NAV_ITEMS = [
  ["首页", "#home"], ["电商智能体", "#agents"], ["供应链与数据", "#supply-chain"],
  ["解决方案", "#solutions"], ["服务生态", "#services"], ["应用实践", "#practices"], ["价格与服务", "#pricing"],
] as const;
const NODE_ASSETS = {"imgNode": {"src": "/sumi/figma-home-hero-imgNode.svg", "width": 27, "height": 27}, "imgNode1": {"src": "/sumi/figma-home-hero-imgNode1.svg", "width": 27, "height": 27}, "imgNode2": {"src": "/sumi/figma-home-hero-imgNode2.svg", "width": 27, "height": 27}, "imgNode3": {"src": "/sumi/figma-home-hero-imgNode3.svg", "width": 27, "height": 27}};
const CAPABILITIES = [
  ["293:211", "01", "AI智能体", "驱动智能决策", "#agents", "imgNode"],
  ["293:232", "02", "全球商品数据", "汇聚全球商品信息", "#data", "imgNode1"],
  ["293:253", "03", "供应链货盘", "链接优质供应资源", "#supply-chain", "imgNode2"],
  ["293:274", "04", "生态服务", "整合商业服务能力", "#services", "imgNode3"],
] as const;

export function MarketingHero({ loginHref }: { loginHref: string }) {
  return <>
    <header className={styles.header} data-node-id="36:3"><div className={styles.headerInner}>
      <a aria-label="硕米智能引擎首页" className={styles.brand} href="#home">
        <Image alt="" height={44} width={44} preload src="/sumi/c3ea4c6f-992f-4c3d-ae2e-c14d4ec9735a.png" /><span>硕米智能引擎</span>
      </a>
      <nav aria-label="官网导航" className={styles.nav}>{NAV_ITEMS.map(([label,href]) => <a href={href} key={href}>{label}</a>)}</nav>
      <div className={styles.headerActions}><Link className={styles.navCta} href={loginHref}>进入硕米</Link>
        <details className={styles.mobileMenu} onClick={event => { if (event.target instanceof HTMLAnchorElement) event.currentTarget.open = false; }}>
          <summary aria-label="展开官网导航"><Menu size={20} /></summary>
          <nav aria-label="移动端官网导航">{NAV_ITEMS.map(([label,href]) => <a href={href} key={href}>{label}</a>)}</nav>
        </details>
      </div>
    </div></header>
    <section className={styles.hero} data-node-id="37:2" id="home">
      <Image className={styles.background} src="/sumi/fd824975-1e65-4585-9ebf-212d68cb1507.png" alt="" fill sizes="100vw" preload />
      <div className={styles.heroInner}>
        <div className={styles.heroCopy} data-node-id="37:3">
          <p className={styles.eyebrow}>新一代 AI 电商智能操作系统</p>
          <h1 className={styles.title}><span>硕米智能引擎</span><span>新一代AI电商</span><span>智能操作系统</span></h1>
          <p className={styles.description}>连接AI智能体、全球商品数据、供应链资源与专业服务，<br />让个人和组织拥有一支可执行、可协同、可增长的智能电商团队。</p>
          <div className={styles.actions}><a className={styles.primaryAction} href="#agents">了解平台能力 <ArrowRight size={16} aria-hidden="true" /></a><a className={styles.secondaryAction} href="#solutions">查看解决方案</a></div>
          <p className={styles.capabilitySummary}>AI智能体 · 全球数据 · 商品供应链 · 专业服务生态</p>
        </div>
        <div className={styles.network} role="group" aria-label="全球商业智能网络能力示意">
          <MotionLayer className={styles.outerRing} nodeId="243:203"><Image src="/sumi/figma-home-hero-img.svg" alt="" width={588} height={328} unoptimized /></MotionLayer>
<MotionLayer className={styles.innerRing} nodeId="243:204"><Image src="/sumi/figma-home-hero-img1.svg" alt="" width={420} height={226} unoptimized /></MotionLayer>
<MotionLayer className={styles.coreGlow} nodeId="288:202"><Image src="/sumi/figma-home-hero-img2.svg" alt="" width={320} height={210} unoptimized /></MotionLayer>
<MotionLayer className={styles.coreDot} nodeId="243:205"><Image src="/sumi/figma-home-hero-img3.svg" alt="" width={62} height={62} unoptimized /></MotionLayer>
          <div className={styles.coreAnchor}><MotionLayer nodeId="288:203" className={styles.core}><small>硕米智能引擎</small><strong>全球商业智能网络</strong><span>四大能力协同</span></MotionLayer></div>
          <ul className={styles.capabilities}>{CAPABILITIES.map(([id,number,title,detail,href,key],index) => <li key={id} className={styles['node'+index]}><MotionLayer nodeId={id} className={styles.nodeMotion}><a href={href} className={styles.capability}><span>{number}</span><div><strong>{title}</strong><small>{detail}</small></div><Image className={styles.nodeGlow} src={NODE_ASSETS[key].src} width={NODE_ASSETS[key].width} height={NODE_ASSETS[key].height} unoptimized alt="" /></a></MotionLayer></li>)}</ul>
        </div>
      </div>
    </section>
  </>;
}
