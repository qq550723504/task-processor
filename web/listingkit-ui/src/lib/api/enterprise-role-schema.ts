import { z } from "zod";
export const enterpriseRoleKey = z.string().regex(/^sumi_role_[a-f0-9]{32}_(0[1-9]|[1-5][0-9]|6[0-4])$/);
export const assignableRole = z.union([z.literal("listingkit_admin"), enterpriseRoleKey]);
export const moduleIds = z.array(z.string().regex(/^[a-z][a-z-]{0,39}$/)).max(32).refine(v => new Set(v).size === v.length);
export const roleDefinition = z.object({ id: assignableRole, name: z.string().min(1).max(160), modules: moduleIds, version: z.number().int().min(0).max(1e9), system: z.boolean() }).strict().refine(v => v.system ? v.id === "listingkit_admin" && v.version === 0 : v.id !== "listingkit_admin" && v.version > 0);
export type EnterpriseRole = z.infer<typeof roleDefinition>;
