"use client";

import { useRef, useState, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsoleState } from "../console/console-page";
import styles from "./task-center.module.css";

// UI slots, not a wire DTO or a source-to-work projection. The consumer supplies
// content only after #340's typed client has validated the response.
type Entry = { key: string; heading: string; caption: string; content: ReactNode; detail: ReactNode };
export function CompletedWorkResults({ entries, pagination }: { entries: readonly Entry[]; pagination?: ReactNode }) {
  // A changed result page starts a fresh selection, including A → empty → A.
  // The data consumer additionally keys the whole request by identity/org/roles.
  return <SelectableWorkResults key={JSON.stringify(entries.map((entry) => entry.key))} entries={entries} pagination={pagination} />;
}

function SelectableWorkResults({ entries, pagination }: { entries: readonly Entry[]; pagination?: ReactNode }) {
  const [selected, setSelected] = useState<string>();
  const detailHeading = useRef<HTMLHeadingElement>(null);
  const rowButtons = useRef(new Map<string, HTMLButtonElement>());
  const current = entries.find((entry) => entry.key === selected);
  return <div className={styles.columns}>
    <Card className={styles.list}>
      <header className={styles.panelHeader}><h2>已完成工作记录</h2><span>仅本地资料准备</span></header>
      {pagination}
      {entries.length ? <ul aria-label="已完成工作记录" className={styles.rows}>{entries.map((entry) => <li key={entry.key}>
        <button type="button" className={styles.row} aria-pressed={current?.key === entry.key}
          ref={(element) => { if (element) rowButtons.current.set(entry.key, element); else rowButtons.current.delete(entry.key); }}
          onClick={() => { setSelected(entry.key); detailHeading.current?.focus(); }}>
          <span className={styles.rowMarker} aria-hidden="true" />
          <span className={styles.rowMain}><strong>{entry.heading}</strong><span className={styles.completed}>通用业务 · 本地资料已创建</span><span>{entry.caption}</span>{entry.content}</span>
          <span className={styles.rowAction}>查看详情 ›</span>
        </button>
      </li>)}</ul> : <ConsoleState kind="empty" title="当前授权范围内暂无本地资料准备记录">已读取当前范围；这不表示企业没有其他业务任务。</ConsoleState>}
    </Card>
    <Card className={styles.detail} role="region" aria-label="工作记录详情">
      <header className={styles.panelHeader}><h2 ref={detailHeading} tabIndex={-1}>工作记录详情</h2><span>仅查看</span></header>
      {current ? <><Button size="sm" variant="outline" onClick={() => { setSelected(undefined); rowButtons.current.get(current.key)?.focus(); }}>返回记录列表</Button>{current.detail}</>
        : <ConsoleState kind="empty" title="选择一条工作记录">从左侧列表查看结果，再打开诊断。诊断将重新校验访问权限。</ConsoleState>}
    </Card>
  </div>;
}

export function WorkResultDetail({ title, summary, children }: { title: string; summary: string; children: ReactNode }) {
  return <div className={styles.detailBody}><h3>{title}</h3><p className={styles.completed}>本地资料已创建 · 通用业务</p><p>{summary}</p>
    <div className={styles.result}><h4>工作结果</h4>{children}</div>
    <p className={styles.note}>资料准备完成不代表诊断通过或可发布。诊断是独立检查。</p>
    <section className={styles.advice}><h4>硕米建议</h4><p>暂未接入</p></section>
  </div>;
}
