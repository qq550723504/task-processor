import Link from "next/link";

import styles from "./legal-document.module.css";

type LegalSection = {
  heading: string;
  paragraphs: string[];
};

export type LegalDocumentProps = {
  title: string;
  summary: string;
  sections: LegalSection[];
};

const policyLinks = [
  ["隐私政策", "/privacy-policy"],
  ["用户协议", "/user-agreement"],
  ["算力计费说明", "/ai-compute-billing"],
  ["服务协议", "/service-agreement"],
] as const;

export function LegalDocument({ title, summary, sections }: LegalDocumentProps) {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <Link href="/" className={styles.brand}>硕米智能引擎</Link>
        <Link href="/" className={styles.back}>返回官网</Link>
      </header>
      <article className={styles.document}>
        <p className={styles.eyebrow}>硕米智能引擎</p>
        <h1>{title}</h1>
        <p className={styles.summary}>{summary}</p>
        <p className={styles.updated}>最后更新：2026 年 8 月 28 日</p>
        <nav className={styles.navigation} aria-label="政策文档">
          {policyLinks.map(([label, href]) => <Link href={href} key={href} aria-current={label === title ? "page" : undefined}>{label}</Link>)}
        </nav>
        {sections.map(({ heading, paragraphs }) => (
          <section key={heading}>
            <h2>{heading}</h2>
            {paragraphs.map((paragraph) => <p key={paragraph}>{paragraph}</p>)}
          </section>
        ))}
        <aside className={styles.notice} aria-label="运营信息及争议处理">
          <p><strong>运营主体：</strong>武汉市硕米科技有限公司</p>
          <p><strong>联系地址：</strong>武汉市洪山区吴家湾大厦 1808</p>
          <p><strong>联系邮箱：</strong><a href="mailto:support@shuomiai.com">support@shuomiai.com</a></p>
          <p><strong>适用法律与争议处理：</strong>本文件适用中华人民共和国法律。因本文件引起或与本文件有关的争议，双方应先友好协商；协商不成的，任何一方可向依法有管辖权的人民法院提起诉讼。</p>
        </aside>
      </article>
    </main>
  );
}
