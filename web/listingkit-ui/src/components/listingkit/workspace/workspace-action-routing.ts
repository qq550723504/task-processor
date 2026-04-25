import type {
  NavigationDispatchPlan,
  NavigationTarget,
  RecoveryDescriptor,
  ResolvedActionSummary,
} from "@/lib/types/listingkit";

function firstDispatchStep(plan?: NavigationDispatchPlan | null) {
  return plan?.steps?.find((step) => step.kind);
}

function isActionableSummary(summary?: ResolvedActionSummary | null) {
  if (!summary) {
    return false;
  }
  if (summary.action_target || summary.action_key) {
    return true;
  }
  return summary.navigation_target?.dispatch_kind === "action";
}

export function pickWorkspaceResolvedActionSummary(
  sessionSummary?: ResolvedActionSummary | null,
  overviewSummary?: ResolvedActionSummary | null,
) {
  if (!sessionSummary) {
    return overviewSummary;
  }
  if (!overviewSummary) {
    return sessionSummary;
  }
  if (!isActionableSummary(sessionSummary) && isActionableSummary(overviewSummary)) {
    return overviewSummary;
  }
  return sessionSummary;
}

export function deriveRecoveryNavigationTarget(
  descriptor: Pick<
    RecoveryDescriptor,
    "descriptor" | "recovery_dispatch_plan" | "recovery_target"
  >,
): NavigationTarget | undefined {
  if (descriptor.recovery_target) {
    return descriptor.recovery_target;
  }

  const primaryStep =
    firstDispatchStep(descriptor.recovery_dispatch_plan) ??
    firstDispatchStep(descriptor.descriptor?.dispatch_plan);
  if (!primaryStep?.kind) {
    return undefined;
  }

  return {
    dispatch_kind: primaryStep.kind,
    descriptor: descriptor.descriptor,
    queue_query: primaryStep.kind === "queue" ? primaryStep.query : undefined,
    session_query: primaryStep.kind === "session" ? primaryStep.query : undefined,
    preview_query: primaryStep.kind === "preview" ? primaryStep.query : undefined,
  };
}
