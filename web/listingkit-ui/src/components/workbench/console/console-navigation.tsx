"use client";

import Link from "next/link";
import { useId, useState } from "react";
import { consoleNavigation, findConsoleRoute, type ConsoleNavNode } from "@/lib/workbench/console-navigation";

export function ConsoleNavigation({ pathname, ariaLabel, onNavigate, productAcquisitionAvailable = false, knowledgeAvailable = false, aiWorkbenchAvailable = false, productReviewAvailable = false, ecoservicesAvailable = false, sheinRecordsAvailable = false }: { pathname: string; ariaLabel: string; onNavigate?: () => void; productAcquisitionAvailable?: boolean; knowledgeAvailable?: boolean; aiWorkbenchAvailable?: boolean; productReviewAvailable?: boolean; ecoservicesAvailable?: boolean; sheinRecordsAvailable?: boolean }) {
  const trail = findConsoleRoute(pathname)?.trail ?? [];
  const navigation = productAcquisitionAvailable ? consoleNavigation : consoleNavigation.map(node => node.href === "/workbench/supply" ? { ...node, children: node.children?.filter(child => child.href !== "/workbench/supply/acquisition") } : node);
  const independentAIEntries: ConsoleNavNode[] = [];
  if (productReviewAvailable) independentAIEntries.push({ label: "标题审核", href: "/workbench/ai/tasks/pending/other", availability: "connected" });
  if (sheinRecordsAvailable) independentAIEntries.push({ label: "历史工作记录", href: "/workbench/ai/tasks/completed/history", availability: "connected" });
  const independentTaskAvailable = productReviewAvailable || sheinRecordsAvailable;
  const nodes = navigation.map(node => {
    if(node.href==="/workbench/services"&&ecoservicesAvailable)return {...node,availability:"connected" as const,children:node.children?.map(child=>({...child,availability:"connected" as const}))};
    if (node.href !== "/workbench/ai") return node;
    const children = (node.children ?? [])
      .filter(child => aiWorkbenchAvailable || child.href !== "/workbench/ai/chat" && (child.href !== "/workbench/ai/tasks" || independentTaskAvailable))
      .map(child => child.href === "/workbench/ai/tasks" && !aiWorkbenchAvailable ? {
        ...child,
        children: child.children?.filter(entry => entry.href === "/workbench/ai/tasks/pending" && productReviewAvailable || entry.href === "/workbench/ai/tasks/completed" && sheinRecordsAvailable),
      } : child.href === "/workbench/ai/knowledge" && knowledgeAvailable ? { ...child, availability: "connected" as const } : child);
    return { ...node, children: [...children, ...independentAIEntries] };
  });
  return <nav aria-label={ariaLabel} className="console-nav"><ul>{nodes.map(node => <NavBranch key={`${pathname}:${node.href}`} node={node} pathname={pathname} trail={trail.map((item) => item.href)} depth={1} onNavigate={onNavigate} />)}</ul></nav>;
}

function NavBranch({ node, pathname, trail, depth, onNavigate }: { node: ConsoleNavNode; pathname: string; trail: readonly string[]; depth: number; onNavigate?: () => void }) {
  const active = trail.includes(node.href) || pathname === node.href;
  // The route-keyed subtree resets disclosure even when navigating back to an earlier route.
  const [expanded, setExpanded] = useState(active);
  const id = useId();
  return <li className={`console-nav-level-${depth}`}>
    <div className="console-nav-row" data-active={active || undefined}>
      <Link href={node.href} prefetch={false} aria-current={pathname === node.href || (!node.children && active) ? "page" : undefined} onClick={onNavigate} title={node.availability === "unavailable" ? `${node.label}：业务暂未启用` : node.label}>
        <span className="console-nav-dot" aria-hidden="true" /><span>{node.label}</span>
      </Link>
      {node.children ? <button type="button" aria-label={`${expanded ? "收起" : "展开"}${node.label}`} aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded(value => !value)}>{expanded ? "⌃" : "⌄"}</button> : null}
    </div>
    {node.children && expanded ? <ul id={id}>{node.children.map((child) => <NavBranch key={child.href} node={child} pathname={pathname} trail={trail} depth={depth + 1} onNavigate={onNavigate} />)}</ul> : null}
  </li>;
}
