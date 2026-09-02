const createRoles = new Set([
  "listingkit_operator",
  "listingkit_admin",
  "platform_admin",
]);
const updateRoles = new Set([
  "listingkit_operator",
  "listingkit_admin",
  "platform_admin",
]);
const deleteRoles = new Set(["listingkit_admin", "platform_admin"]);

export function canCreateWorkbenchStore(roles: readonly string[]) {
  return roles.some((role) => createRoles.has(role));
}

export function canUpdateWorkbenchStore(roles: readonly string[]) {
  return roles.some((role) => updateRoles.has(role));
}

export function canDeleteWorkbenchStore(roles: readonly string[]) {
  return roles.some((role) => deleteRoles.has(role));
}
