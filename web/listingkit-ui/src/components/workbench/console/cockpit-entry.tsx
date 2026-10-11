"use client";

import { redirect } from "next/navigation";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { firstConnectedConsoleEntry } from "@/lib/workbench/console-primary-entry";
import { ConsoleOverview } from "./console-overview";
import { ConsoleState } from "./console-page";

export function CockpitEntry() {
  const context = useWorkbenchContext();
  if (context.isLoading || context.isSwitching || context.selectionRequired || !context.effectiveOrganization || context.error || context.blockingError) {
    return <ConsoleState kind="loading" title="正在读取工作台..." />;
  }
  if (!context.operationsCockpitAvailable) return <ConsoleOverview />;
  const destination = firstConnectedConsoleEntry("/workbench", { operationsCockpitAvailable: context.operationsCockpitAvailable, cockpitPermissions: context.permissions });
  if (destination) return redirect(destination);
  return <ConsoleState kind="error" title="没有运营驾驶舱查看权限">请联系当前企业管理员确认模块权限。</ConsoleState>;
}
