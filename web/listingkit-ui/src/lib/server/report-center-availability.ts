// Deployment availability is independent of the live Go authorization decision.
export function isReportCenterAvailable() {
  return process.env.LISTINGKIT_REPORT_CENTER_ENABLED === "true";
}
