import Link from "next/link";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils/cn";

export function ConsolePage({ title, description, breadcrumbs, actions, children, className }: { title: string; description?: ReactNode; breadcrumbs?: readonly { label: string; href?: string }[]; actions?: ReactNode; children: ReactNode; className?: string }) {
  return <section className={cn("console-page", className)}>
    {breadcrumbs?.length ? <nav aria-label="面包屑" className="console-breadcrumb"><ol>{breadcrumbs.map((item, index) => <li key={`${item.label}:${index}`}>{index > 0 ? <span aria-hidden="true"> / </span> : null}{item.href && index < breadcrumbs.length - 1 ? <Link href={item.href} prefetch={false}>{item.label}</Link> : <span aria-current={index === breadcrumbs.length - 1 ? "page" : undefined}>{item.label}</span>}</li>)}</ol></nav> : null}
    <header className="console-page-header"><div className="min-w-0"><h1>{title}</h1>{description ? <div className="console-description">{description}</div> : null}</div>{actions ? <div className="console-page-actions" role="group" aria-label="页面操作">{actions}</div> : null}</header>
    {children}
  </section>;
}

export function ConsoleToolbar({ children }: { children: ReactNode }) {
  return <Card className="console-toolbar" role="group" aria-label="列表筛选">{children}</Card>;
}

export function ConsoleState({ kind, title, children }: { kind: "loading" | "empty" | "error" | "unavailable"; title: string; children?: ReactNode }) {
  return <Card className="console-state" data-state={kind} role={kind === "error" ? "alert" : "status"}><h2>{title}</h2>{children ? <div className="console-description">{children}</div> : null}</Card>;
}
