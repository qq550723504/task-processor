import type { ReactNode } from "react";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";
import { ConsolePage } from "../console/console-page";
import styles from "./account.module.css";

/** Shared presentation only. Each account leaf retains its own authorization and data reader. */
export function AccountShell({ pathname, title, description, actions, children }: {
  pathname: string;
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const trail = findConsoleRoute(pathname)?.trail;
  return <ConsolePage title={title} description={description} actions={actions} className={styles.page}
    breadcrumbs={trail?.map(node => ({ label: node.label, href: node.href }))}>{children}</ConsolePage>;
}
