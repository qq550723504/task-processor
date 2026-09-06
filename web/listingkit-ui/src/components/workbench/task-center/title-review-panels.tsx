"use client";

import { useRef, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsoleState } from "../console/console-page";
import styles from "./task-center.module.css";
import reviewStyles from "./title-review.module.css";

// Render slots are not wire DTOs. Selection and validated data belong to the consumer.
type Entry = { key: string; heading: string; caption: string; statusLabel: string; accepted: boolean; metadata: ReactNode };
export function TitleReviewPanels({ entries, selectedKey, detail, pagination, onSelect, onClose, busy = false }: {
  entries: readonly Entry[]; selectedKey?: string; detail?: ReactNode; pagination?: ReactNode;
  onSelect: (key: string) => void; onClose: () => void; busy?: boolean;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  const listHeading = useRef<HTMLHeadingElement>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  return <div className={styles.columns}>
    <Card className={styles.list}>
      <header className={styles.panelHeader}><h2 ref={listHeading} tabIndex={-1}>待处理标题提案</h2><span>标准商品 · 仅标题</span></header>
      {pagination}
      {entries.length ? <ul aria-label="待处理标题提案" className={styles.rows}>{entries.map((entry) => <li key={entry.key}>
        <button type="button" className={styles.row} disabled={busy} aria-pressed={selectedKey === entry.key}
          ref={(element) => { if (element) buttons.current.set(entry.key, element); else buttons.current.delete(entry.key); }}
          onClick={() => { onSelect(entry.key); heading.current?.focus(); }}>
          <span aria-hidden="true" className={`${styles.rowMarker} ${entry.accepted ? reviewStyles.acceptedMarker : reviewStyles.pendingMarker}`} />
          <span className={styles.rowMain}><strong>{entry.heading}</strong><span className={entry.accepted ? reviewStyles.accepted : reviewStyles.pending}>{entry.statusLabel}</span><span>{entry.caption}</span>{entry.metadata}</span>
          <span className={styles.rowAction}>查看详情 ›</span>
        </button>
      </li>)}</ul> : <ConsoleState kind="empty" title="当前授权范围内暂无待处理标题提案">已读取当前范围。已应用和已拒绝的提案不在此列表中。</ConsoleState>}
    </Card>
    <Card className={styles.detail} role="region" aria-label="标题提案详情">
      <header className={styles.panelHeader}><h2 ref={heading} tabIndex={-1}>标题提案详情</h2></header>
      {selectedKey ? <><Button variant="outline" size="sm" disabled={busy} onClick={() => { onClose(); (buttons.current.get(selectedKey) ?? listHeading.current)?.focus(); }}>返回提案列表</Button>{detail}</>
        : <ConsoleState kind="empty" title="选择一条标题提案">查看修改前后、原始建议与依据，再决定接受、编辑或拒绝。</ConsoleState>}
    </Card>
  </div>;
}
