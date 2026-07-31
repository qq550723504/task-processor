export const PLATFORM_ADMIN_ROLES = ["platform_admin", "admin"] as const;

export function hasPlatformAdminRole(roles?: readonly string[]) {
  return (roles ?? []).some((role) => PLATFORM_ADMIN_ROLES.includes(role as (typeof PLATFORM_ADMIN_ROLES)[number]));
}
