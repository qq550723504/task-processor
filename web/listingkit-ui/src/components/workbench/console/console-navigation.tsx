"use client";

import Link from "next/link";
import { useId, useState } from "react";
import { consoleNavigation, findConsoleRoute, type ConsoleNavNode } from "@/lib/workbench/console-navigation";

export function ConsoleNavigation({ pathname, ariaLabel, onNavigate }: { pathname: string; ariaLabel: string; onNavigate?: () => void }) {
  const trail = findConsoleRoute(pathname)?.trail ?? [];
  return <nav aria-label={ariaLabel} className="console-nav"><ul>{consoleNavigation.map((node) => <NavBranch key={node.href} node={node} pathname={pathname} trail={trail.map((item) => item.href)} depth={1} onNavigate={onNavigate} />)}</ul></nav>;
}

function NavBranch({ node, pathname, trail, depth, onNavigate }: { node: ConsoleNavNode; pathname: string; trail: readonly string[]; depth: number; onNavigate?: () => void }) {
  const active = trail.includes(node.href);
  const [disclosure, setDisclosure] = useState({ pathname, expanded: active });
  // A route transition reveals its selected branch; manual collapse lasts for that route.
  const expanded = disclosure.pathname === pathname ? disclosure.expanded : active;
  const id = useId();
  return <li className={`console-nav-level-${depth}`}>
    <div className="console-nav-row" data-active={active || undefined}>
      <Link href={node.href} prefetch={false} aria-current={pathname === node.href || (!node.children && active) ? "page" : undefined} onClick={onNavigate} title={node.availability === "unavailable" ? `${node.label}：业务暂未启用` : node.label}>
        <span className="console-nav-dot" aria-hidden="true" /><span>{node.label}</span>
      </Link>
      {node.children ? <button type="button" aria-label={`${expanded ? "收起" : "展开"}${node.label}`} aria-expanded={expanded} aria-controls={id} onClick={() => setDisclosure({ pathname, expanded: !expanded })}>{expanded ? "⌃" : "⌄"}</button> : null}
    </div>
    {node.children && expanded ? <ul id={id}>{node.children.map((child) => <NavBranch key={child.href} node={child} pathname={pathname} trail={trail} depth={depth + 1} onNavigate={onNavigate} />)}</ul> : null}
  </li>;
}
