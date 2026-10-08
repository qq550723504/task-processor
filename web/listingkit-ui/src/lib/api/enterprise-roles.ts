import { z } from "zod";
import { memberId, MemberError, MemberScope, requestMembers } from "./members";

import { enterpriseRoleKey, moduleIds, roleDefinition } from "./enterprise-role-schema";
export { enterpriseRoleKey, assignableRole, roleDefinition } from "./enterprise-role-schema";
export type { EnterpriseRole } from "./enterprise-role-schema";
export const createRoleInput = z.object({ name: z.string().trim().refine(v => [...v].length > 0 && [...v].length <= 40 && !/\p{Cc}/u.test(v)), modules: moduleIds }).strict();
export const saveRoleInput = z.object({ modules: moduleIds, expectedVersion: z.number().int().min(1).max(1e9) }).strict();
const catalogItem = z.object({ id: z.string().max(40), group: z.string().max(80), label: z.string().max(80), available: z.boolean(), permissions: z.array(z.string().max(128)).max(128) }).strict();
const roleList = z.object({ schemaVersion: z.literal("enterprise-roles-v1"), userId: memberId, organizationId: memberId, items: z.array(roleDefinition).min(1).max(65), catalog: z.array(catalogItem).max(32), canManage: z.boolean(), canCreate: z.boolean(), remainingSlots: z.number().int().min(0).max(64) }).strict().refine(v => new Set(v.items.map(r => r.id)).size === v.items.length && (!v.canCreate || (v.canManage && v.remainingSlots > 0)) && v.items.every(r => r.modules.every(id => v.catalog.some(m => m.id === id && m.available))));
const mutation = z.object({ schemaVersion: z.literal("enterprise-role-mutation-v1"), userId: memberId, organizationId: memberId, operationId: z.string().uuid(), role: roleDefinition }).strict();
export type EnterpriseRoles = z.infer<typeof roleList>;
export function parseEnterpriseRoles(value: unknown) { return roleList.parse(value); }
export function parseRoleMutation(value: unknown, key: string) { const result = mutation.parse(value); if (result.operationId !== key || result.role.system) throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE"); return result; }
export const getEnterpriseRoles = (scope: MemberScope) => requestMembers("/roles", scope, "GET", parseEnterpriseRoles);
export function createEnterpriseRole(scope: MemberScope, key: string, input: z.infer<typeof createRoleInput>) { return requestMembers("/roles", scope, "POST", v => parseRoleMutation(v, key), key, createRoleInput.parse(input)); }
export function saveEnterpriseRole(scope: MemberScope, key: string, id: string, input: z.infer<typeof saveRoleInput>) { return requestMembers(`/roles/${enterpriseRoleKey.parse(id)}/permissions`, scope, "POST", v => parseRoleMutation(v, key), key, saveRoleInput.parse(input)); }
