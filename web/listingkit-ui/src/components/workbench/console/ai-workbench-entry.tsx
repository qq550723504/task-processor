"use client";

import {redirect} from "next/navigation";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {type AIWorkbenchEntryCapabilities} from "@/lib/workbench/ai-workbench-entry";
import {firstConnectedConsoleEntry} from "@/lib/workbench/console-primary-entry";
import {ConsoleUnavailable} from "./console-overview";
import {ConsoleState} from "./console-page";

export function AIWorkbenchEntry(props: Omit<AIWorkbenchEntryCapabilities, "aiWorkbenchAvailable" | "projectCenterAvailable">) {
  const context = useWorkbenchContext();
  if (context.isLoading || context.isSwitching || context.selectionRequired || !context.effectiveOrganization || context.error || context.blockingError) {
    return <ConsoleState kind="loading" title="正在读取工作台..." />;
  }
  const destination = firstConnectedConsoleEntry("/workbench/ai", {...props, aiWorkbenchAvailable: context.aiWorkbenchAvailable, projectCenterAvailable: context.projectCenterAvailable});
  if (destination) return redirect(destination);
  return <ConsoleUnavailable pathname="/workbench/ai" />;
}
