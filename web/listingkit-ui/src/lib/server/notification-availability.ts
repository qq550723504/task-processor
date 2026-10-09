// Deployment switch; original server identity/resource gates still authorize every request.
export function isNotificationCenterAvailable() { return process.env.LISTINGKIT_NOTIFICATION_CENTER_ENABLED === "true"; }
