import Image from "next/image";
import Link from "next/link";
import { ArrowRight } from "lucide-react";

import { HeroSystemVisual } from "./hero-system-visual";
import styles from "./marketing-hero.module.css";

const ASSET = "/sumi";

const NAV_ITEMS = [
  ["产品架构", "#architecture"],
  ["能力中心", "#agents"],
  ["场景方案", "#solutions"],
] as const;

export function MarketingHero({ loginHref }: { loginHref: string }) {
  return (
    <>
      <header className={styles.header}>
        <div className={styles.headerInner}>
          <Link
            aria-label="硕米智能引擎首页"
            className={styles.brand}
            href="#home"
          >
            <Image
              alt=""
              height={34}
              priority
              src={`${ASSET}/a5e50b9c-bfa0-4012-bb43-e53ee1a4ed17.png`}
              width={34}
            />
            <span>硕米智能引擎</span>
          </Link>

          <nav aria-label="官网导航" className={styles.nav}>
            {NAV_ITEMS.map(([label, href]) => (
              <a href={href} key={href}>
                {label}
              </a>
            ))}
          </nav>

          <Link className={styles.navCta} href={loginHref}>
            进入系统
          </Link>
        </div>
      </header>

      <section
        className={styles.hero}
        data-motion-sequence="boot-reveal-active-pulse"
        data-node-id="10:101"
        id="home"
      >
        <div aria-hidden="true" className={styles.ambientGlow} />
        <div className={styles.heroInner}>
          <div className={styles.heroCopy} data-node-id="10:132">
            <p className={styles.eyebrow}>
              <span aria-hidden="true" className={styles.liveDot} />
              AI-NATIVE COMMERCE OPERATING SYSTEM
            </p>

            <h1 className={styles.title}>
              <span>让智能，成为电商经营的</span>
              <span>默认能力</span>
            </h1>

            <p className={styles.description}>
              连接 Agent、商品数据、供应链与平台执行能力，让选品、内容生产、平台适配和店铺增长在一个系统中持续协同。
            </p>

            <div className={styles.actions}>
              <Link className={styles.primaryAction} href={loginHref}>
                进入硕米 OS
                <ArrowRight aria-hidden="true" size={17} />
              </Link>
              <a className={styles.secondaryAction} href="#architecture">
                查看系统架构
              </a>
            </div>

            <p className={styles.microNote}>
              一个入口 · 多智能体协同 · 全链路可观测
            </p>
          </div>

          <HeroSystemVisual />
        </div>

        <div className={styles.systemStatus} data-node-id="10:195">
          <p>
            <span aria-hidden="true" className={styles.statusDot} />
            SYSTEM ONLINE · AI COMMERCE FABRIC ACTIVE
          </p>
          <p>Agent · 商品 · 工具 · 平台执行能力已连接</p>
        </div>
      </section>
    </>
  );
}
