"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";
import { ConsolePage, ConsoleState, ConsoleToolbar } from "../console/console-page";
import styles from "./task-center.module.css";

export function TaskCenterLayout({ completed = false, onRefresh, children }: { completed?: boolean; onRefresh?: () => void; children?: ReactNode }) {
  const pathname = completed ? "/workbench/ai/tasks/completed" : "/workbench/ai/tasks";
  return <ConsolePage title="任务中心" breadcrumbs={findConsoleRoute(pathname)?.trail}
    description="查看当前已接入的业务工作记录。" className={styles.page}
    actions={<><Button variant="outline" disabled>搜索任务 · 暂未接入</Button><Button disabled>向硕米发起任务 · 暂未接入</Button></>}>
    <div className={styles.metrics}>
      {["待你处理", "执行中", "今日完成"].map((title) => <Card className={styles.metric} key={title}><span className={styles.swatch} aria-hidden="true" /><div><h2>{title}</h2><p>统计暂未接入</p></div></Card>)}
      <Card className={styles.scope}><p>工作范围</p><h2>通用业务</h2><p>当前企业 · 店铺筛选暂未接入</p></Card>
    </div>
    <ConsoleToolbar>
      <nav aria-label="任务状态" className={styles.filters}>
        <Link href="/workbench/ai/tasks" prefetch={false} aria-current={!completed ? "page" : undefined}>全部</Link>
        <Button variant="outline" disabled>待你处理 · 未接入</Button>
        <Button variant="outline" disabled>执行中 · 未接入</Button>
        <Link href="/workbench/ai/tasks/completed" prefetch={false} aria-current={completed ? "page" : undefined}>已完成</Link>
        <Button variant="outline" disabled>已暂停 · 未接入</Button>
      </nav>
      {onRefresh ? <Button variant="outline" onClick={onRefresh}>刷新记录</Button> : null}
    </ConsoleToolbar>
    <p className={styles.coverage}>仅覆盖本地资料准备完成记录，并非企业全部任务。“完成”指本地资料创建已提交，不代表诊断通过或可发布。</p>
    {children ?? <ConsoleState kind="unavailable" title="暂未启用">当前环境尚未接入工作记录读取能力，未查询业务数据。</ConsoleState>}
  </ConsolePage>;
}
