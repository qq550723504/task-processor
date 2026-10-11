"use client";

import Link from "next/link";
import {useSearchParams} from "next/navigation";
import {collectionID} from "@/lib/contracts/product-collection";
import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ecoPageSchema, ecoRequest } from "@/lib/api/ecoservices";
import { listMarketRecords } from "@/lib/api/supply-market";
import { consoleNavigation, findConsoleRoute, type ConsoleNavNode } from "@/lib/workbench/console-navigation";
import { firstConnectedAIEntry } from "@/lib/workbench/ai-workbench-entry";

export function ConsoleNavigation({ pathname, ariaLabel, onNavigate, dataServicesAvailable = false, reportCenterAvailable = false, toolMarketAvailable = false, supplyMarketAvailable = false, productAcquisitionAvailable = false, productCollectionsAvailable = false, supplyChainAvailable = false, knowledgeAvailable = false, projectCenterAvailable = false, operationsCockpitAvailable = false, cockpitPermissions = [], storeProductsAvailable = false, storeOrdersAvailable = false, aiWorkbenchAvailable = false, productReviewAvailable = false, ecoservicesAvailable = false, sheinRecordsAvailable = false, userId }: { pathname: string; ariaLabel: string; onNavigate?: () => void; dataServicesAvailable?: boolean; reportCenterAvailable?: boolean; toolMarketAvailable?: boolean; supplyMarketAvailable?: boolean; productAcquisitionAvailable?: boolean; productCollectionsAvailable?: boolean; supplyChainAvailable?: boolean; knowledgeAvailable?: boolean; projectCenterAvailable?: boolean; operationsCockpitAvailable?: boolean; cockpitPermissions?: readonly string[]; storeProductsAvailable?: boolean; storeOrdersAvailable?: boolean; aiWorkbenchAvailable?: boolean; productReviewAvailable?: boolean; ecoservicesAvailable?: boolean; sheinRecordsAvailable?: boolean; userId?: string }) {
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
    if (node.href === "/workbench" && operationsCockpitAvailable) return { ...node, availability: "connected" as const, children: node.children?.map(child => {
      const mode = child.href.split("/").at(-1), permission = mode === "stores" ? "workbench.cockpit.stores.read" : `workbench.cockpit.${mode}.read`;
      return { ...child, availability: cockpitPermissions.includes(permission) ? "connected" as const : "unavailable" as const };
    }) };
    if (node.href === "/workbench/tools" && toolMarketAvailable) return {...node, availability:"connected" as const, children:node.children?.map(child=>({...child, availability:"connected" as const}))};
 if(node.href === "/workbench/store-center") return {...node,children:node.children?.map(child => child.href === "/workbench/store-products" ? {...child,availability:storeProductsAvailable ? "connected" as const:"unavailable" as const} : child.href === "/workbench/store-orders" ? {...child,availability:storeOrdersAvailable ? "connected" as const:"unavailable" as const} :child)};
    if(node.href==="/workbench/services"&&ecoservicesAvailable)return {...node,availability:"connected" as const,children:node.children?.map(child=>({...child,availability:"connected" as const}))};
    if (node.href === "/workbench/supply") return { ...node, children: node.children?.map(child => child.href === "/workbench/supply/mine" ? { ...child, availability: supplyChainAvailable ? "connected" as const : "unavailable" as const, children: supplyChainAvailable ? child.children?.map(stage=>({...stage,availability:"connected" as const})) : undefined } : ["/workbench/supply/official","/workbench/supply/selected","/workbench/supply/catalogs","/workbench/supply/applications"].includes(child.href) && supplyMarketAvailable ? {...child,availability:"connected" as const}:child) };
    if (node.href === "/workbench/data") return { ...node, availability: dataServicesAvailable || productCollectionsAvailable ? "connected" as const : node.availability, children: node.children?.map(child => child.href === "/workbench/data/mine" && productCollectionsAvailable || ["/workbench/data/market", "/workbench/data/api"].includes(child.href) && dataServicesAvailable ? { ...child, availability: "connected" as const } : child) };
    if (node.href !== "/workbench/ai") return node;
    const children = (node.children ?? [])
      .filter(child => aiWorkbenchAvailable || child.href !== "/workbench/ai/chat" && (child.href !== "/workbench/ai/tasks" || independentTaskAvailable))
      .map(child => child.href === "/workbench/ai/reports" ? {...child, availability:reportCenterAvailable ? "connected" as const:"unavailable" as const, children:reportCenterAvailable ? child.children?.map(entry=>({...entry,availability:"connected" as const})):undefined} : child.href === "/workbench/ai/projects" ? {...child,availability:projectCenterAvailable?"connected" as const:"unavailable" as const,children:projectCenterAvailable?child.children:undefined} : child.href === "/workbench/ai/tasks" && !aiWorkbenchAvailable ? {
        ...child,
        children: child.children?.filter(entry => entry.href === "/workbench/ai/tasks/pending" && productReviewAvailable || entry.href === "/workbench/ai/tasks/completed" && sheinRecordsAvailable),
      } : child.href === "/workbench/ai/knowledge" && knowledgeAvailable ? { ...child, availability: "connected" as const } : child);
    const connected = firstConnectedAIEntry({aiWorkbenchAvailable, projectCenterAvailable, knowledgeAvailable, reportCenterAvailable, productReviewAvailable, sheinRecordsAvailable}) !== null;
    return { ...node, availability: connected ? "connected" as const : "unavailable" as const, children: [...children, ...independentAIEntries] };
  });
  return <nav aria-label={ariaLabel} className="console-nav"><ul>{nodes.map(node => supplyMarketAvailable && userId && node.href==="/workbench/supply" ? <MarketBranch key={`${pathname}:${node.href}`} node={node} pathname={pathname} trail={trail.map(item=>item.href)} onNavigate={onNavigate} userId={userId} supplyQuery={supplyQuery}/> : ecoservicesAvailable && userId && node.href === "/workbench/services" ? <EcoservicesBranch key={`${pathname}:${node.href}`} node={node} pathname={pathname} trail={trail.map(item => item.href)} onNavigate={onNavigate} userId={userId} /> : <NavBranch key={`${pathname}:${node.href}`} node={node} pathname={pathname} trail={trail.map((item) => item.href)} depth={1} onNavigate={onNavigate} supplyQuery={supplyQuery} />)}</ul></nav>;
}
function MarketBranch({node,pathname,trail,onNavigate,userId,supplyQuery}:{node:ConsoleNavNode;pathname:string;trail:readonly string[];onNavigate?:()=>void;userId:string;supplyQuery:string}){
 const authorized=useQuery({queryKey:["supply-market",userId,"","records",true,"selected",false,undefined],queryFn:({signal})=>listMarketRecords({userId,organizationId:""},{kind:"selected",ended:false},true,signal),retry:false});
 const branch=authorized.isSuccess&&!authorized.isError ? {...node,children:[...(node.children??[]),{label:"发布管理/处理申请",href:"/workbench/supply/operator",availability:"connected" as const}]}:node;
 const currentTrail=pathname==="/workbench/supply/operator" || pathname.startsWith("/workbench/supply/operator/") ? [...trail,node.href,"/workbench/supply/operator"]:trail;
 return <NavBranch node={branch} pathname={pathname} trail={currentTrail} depth={1} onNavigate={onNavigate} supplyQuery={supplyQuery}/>;
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
