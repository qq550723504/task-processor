export const SHEIN_ENROLLMENT_TABS = [
  "products",
  "costs",
  "candidates",
  "runs",
] as const;

export type SheinEnrollmentTab = (typeof SHEIN_ENROLLMENT_TABS)[number];

export const SHEIN_ACTIVITY_TYPE_OPTIONS = [
  { value: "PROMOTION", label: "促销活动" },
  { value: "TIME_LIMITED", label: "限时活动" },
  { value: "MIXED", label: "混合活动" },
] as const;

export function parseSheinEnrollmentTab(
  value: string | undefined,
): SheinEnrollmentTab {
  if (value === "products" || value === "costs" || value === "candidates" || value === "runs") {
    return value;
  }
  return "candidates";
}

export function parseSheinActivityType(value: string | undefined) {
  if (value === "PROMOTION" || value === "TIME_LIMITED" || value === "MIXED") {
    return value;
  }
  return "PROMOTION";
}

export function sheinEnrollmentTabLabel(tab: SheinEnrollmentTab) {
  switch (tab) {
    case "products":
      return "同步商品";
    case "costs":
      return "成本价维护";
    case "candidates":
      return "候选池";
    case "runs":
      return "报名记录";
  }
}
