export const SHEIN_ENROLLMENT_TABS = ["candidates", "runs"] as const;

export type SheinEnrollmentTab = (typeof SHEIN_ENROLLMENT_TABS)[number];

export const SHEIN_ACTIVITY_TYPE_OPTIONS = [
  { value: "PROMOTION", label: "促销活动" },
  { value: "TIME_LIMITED", label: "限时活动" },
] as const;

export function parseSheinEnrollmentTab(
  value: string | undefined,
): SheinEnrollmentTab {
  if (value === "candidates" || value === "runs") {
    return value;
  }
  return "candidates";
}

export function parseSheinActivityType(value: string | undefined) {
  if (value === "PROMOTION" || value === "TIME_LIMITED") {
    return value;
  }
  return "PROMOTION";
}

export function sheinEnrollmentTabLabel(tab: SheinEnrollmentTab) {
  switch (tab) {
    case "candidates":
      return "候选池";
    case "runs":
      return "报名记录";
  }
}
