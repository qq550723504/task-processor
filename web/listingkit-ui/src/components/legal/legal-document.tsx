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
        <aside className={styles.notice}>
          本文为当前上线版本的基础文本。运营主体名称、联系地址、适用法律与备案信息应在正式上线前由运营及法务团队核对、补充并确认。
        </aside>
      </article>
    </main>
  );
}
