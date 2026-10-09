"use client";

import Link from "next/link";
import {useSearchParams} from "next/navigation";
import {collectionID} from "@/lib/contracts/product-collection";
import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ecoPageSchema, ecoRequest } from "@/lib/api/ecoservices";
import { consoleNavigation, findConsoleRoute, type ConsoleNavNode } from "@/lib/workbench/console-navigation";

export function ConsoleNavigation({ pathname, ariaLabel, onNavigate, toolMarketAvailable = false, productAcquisitionAvailable = false, productCollectionsAvailable = false, supplyChainAvailable = false, knowledgeAvailable = false, aiWorkbenchAvailable = false, productReviewAvailable = false, ecoservicesAvailable = false, sheinRecordsAvailable = false, userId }: { pathname: string; ariaLabel: string; onNavigate?: () => void; toolMarketAvailable?: boolean; productAcquisitionAvailable?: boolean; productCollectionsAvailable?: boolean; supplyChainAvailable?: boolean; knowledgeAvailable?: boolean; aiWorkbenchAvailable?: boolean; productReviewAvailable?: boolean; ecoservicesAvailable?: boolean; sheinRecordsAvailable?: boolean; userId?: string }) {
  const trail = findConsoleRoute(pathname)?.trail ?? [];
  const query=useSearchParams(), selection=new URLSearchParams();
  if(pathname.startsWith("/workbench/supply/mine"))for(const key of ["preparation","store"]){const value=query?.get(key);if(value&&collectionID.safeParse(value).success)selection.set(key,value);}
  const supplyQuery=selection.size?`?${selection}`:"";
  const navigation = productAcquisitionAvailable ? consoleNavigation : consoleNavigation.map(node => node.href === "/workbench/supply" ? { ...node, children: node.children?.filter(child => child.href !== "/workbench/supply/acquisition") } : node);
  const independentAIEntries: ConsoleNavNode[] = [];
  if (productReviewAvailable) independentAIEntries.push({ label: "标题审核", href: "/workbench/ai/tasks/pending/other", availability: "connected" });
  if (sheinRecordsAvailable) independentAIEntries.push({ label: "历史工作记录", href: "/workbench/ai/tasks/completed/history", availability: "connected" });
  const independentTaskAvailable = productReviewAvailable || sheinRecordsAvailable;
  const nodes = navigation.map(node => {
    if (node.href === "/workbench/tools" && toolMarketAvailable) return {...node, availability:"connected" as const, children:node.children?.map(child=>({...child, availability:"connected" as const}))};
    if(node.href==="/workbench/services"&&ecoservicesAvailable)return {...node,availability:"connected" as const,children:node.children?.map(child=>({...child,availability:"connected" as const}))};
    if (node.href === "/workbench/supply") return { ...node, children: node.children?.map(child => child.href === "/workbench/supply/mine" ? { ...child, availability: supplyChainAvailable ? "connected" as const : "unavailable" as const, children: supplyChainAvailable ? child.children?.map(stage=>({...stage,availability:"connected" as const})) : undefined } : child) };
    if (node.href === "/workbench/data" && productCollectionsAvailable) return { ...node, children: node.children?.map(child => child.href === "/workbench/data/mine" ? { ...child, availability: "connected" as const } : child) };
    if (node.href !== "/workbench/ai") return node;
    const children = (node.children ?? [])
      .filter(child => aiWorkbenchAvailable || child.href !== "/workbench/ai/chat" && (child.href !== "/workbench/ai/tasks" || independentTaskAvailable))
      .map(child => child.href === "/workbench/ai/tasks" && !aiWorkbenchAvailable ? {
        ...child,
        children: child.children?.filter(entry => entry.href === "/workbench/ai/tasks/pending" && productReviewAvailable || entry.href === "/workbench/ai/tasks/completed" && sheinRecordsAvailable),
      } : child.href === "/workbench/ai/knowledge" && knowledgeAvailable ? { ...child, availability: "connected" as const } : child);
    return { ...node, children: [...children, ...independentAIEntries] };
  });
  return <nav aria-label={ariaLabel} className="console-nav"><ul>{nodes.map(node => ecoservicesAvailable && userId && node.href === "/workbench/services" ? <EcoservicesBranch key={`${pathname}:${node.href}`} node={node} pathname={pathname} trail={trail.map(item => item.href)} onNavigate={onNavigate} userId={userId} /> : <NavBranch key={`${pathname}:${node.href}`} node={node} pathname={pathname} trail={trail.map((item) => item.href)} depth={1} onNavigate={onNavigate} supplyQuery={supplyQuery} />)}</ul></nav>;
}

function EcoservicesBranch({ node, pathname, trail, onNavigate, userId }: { node: ConsoleNavNode; pathname: string; trail: readonly string[]; onNavigate?: () => void; userId: string }) {
  // Reuse the page's global platform read and cache. Enterprise roles/grants
  // cannot advertise review; the backend's configured platform gate decides.
  const review = useQuery({
    queryKey: ["ecoservices", userId, "", "platform", "applications", 1],
    queryFn: ({ signal }) => ecoRequest({ userId, organizationId: "" }, "applications?page=1&pageSize=20", ecoPageSchema, { signal }, true),
    retry: false,
  });
  const authorized = review.isSuccess && !review.isError;
  const currentTrail = pathname === "/workbench/services/review" ? [...trail, node.href] : trail;
  const branch = authorized ? { ...node, children: [...node.children ?? [], { label: "平台审核", href: "/workbench/services/review", availability: "connected" as const }] } : node;
  return <NavBranch node={branch} pathname={pathname} trail={currentTrail} depth={1} onNavigate={onNavigate} />;
}

function NavBranch({ node, pathname, trail, depth, onNavigate, supplyQuery = "" }: { node: ConsoleNavNode; pathname: string; trail: readonly string[]; depth: number; onNavigate?: () => void; supplyQuery?:string }) {
  const active = trail.includes(node.href) || pathname === node.href;
  // The route-keyed subtree resets disclosure even when navigating back to an earlier route.
  const [expanded, setExpanded] = useState(active);
  const id = useId();
  return <li className={`console-nav-level-${depth}`}>
    <div className="console-nav-row" data-active={active || undefined}>
      <Link href={node.href.startsWith("/workbench/supply/mine")?node.href+supplyQuery:node.href} prefetch={false} aria-current={pathname === node.href || (!node.children && active) ? "page" : undefined} onClick={onNavigate} title={node.availability === "unavailable" ? `${node.label}：业务暂未启用` : node.label}>
        <span className="console-nav-dot" aria-hidden="true" /><span>{node.label}</span>
      </Link>
      {node.children ? <button type="button" aria-label={`${expanded ? "收起" : "展开"}${node.label}`} aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded(value => !value)}>{expanded ? "⌃" : "⌄"}</button> : null}
    </div>
    {node.children && expanded ? <ul id={id}>{node.children.map((child) => <NavBranch key={child.href} node={child} pathname={pathname} trail={trail} depth={depth + 1} onNavigate={onNavigate} supplyQuery={supplyQuery} />)}</ul> : null}
  </li>;
}
