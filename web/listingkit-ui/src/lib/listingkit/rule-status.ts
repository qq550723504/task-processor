export const LISTING_RULE_STATUS_ENABLED = 0;
const LISTING_RULE_STATUS_DISABLED = 1;

export function isListingRuleEnabled(status: number): boolean {
  return status === LISTING_RULE_STATUS_ENABLED;
}

export function nextListingRuleStatus(status: number): number {
  return isListingRuleEnabled(status)
    ? LISTING_RULE_STATUS_DISABLED
    : LISTING_RULE_STATUS_ENABLED;
}
