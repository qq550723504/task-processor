import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";

export function resolveSheinStudioEffectiveStep(args: {
  activeStep: SheinStudioStepKey;
  createdTaskCount?: number;
  designCount?: number;
}) {
  if (args.activeStep === "select" || args.activeStep === "review" || args.activeStep === "tasks") {
    return args.activeStep;
  }

  if ((args.createdTaskCount ?? 0) > 0) {
    return "tasks" satisfies SheinStudioStepKey;
  }
  if ((args.designCount ?? 0) > 0) {
    return "review" satisfies SheinStudioStepKey;
  }
  return args.activeStep;
}
