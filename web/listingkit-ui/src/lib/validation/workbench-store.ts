import { z } from "zod";

const workbenchStorePlatformSchema = z.literal("shein");
const workbenchStoreRecordStatusSchema = z.enum([
  "active",
  "disabled",
  "deleting",
]);

const publicText = (maximumCodePoints: number, required: boolean) =>
  z
    .string()
    .refine((value) => !/\p{Cc}/u.test(value))
    .transform((value) => value.trim())
    .refine(
      (value) =>
        (!required || value.length > 0) &&
        Array.from(value).length <= maximumCodePoints,
    );

export const workbenchStoreCreateSchema = z
  .object({
    name: publicText(120, true),
    platform: workbenchStorePlatformSchema,
    region: publicText(64, true),
    externalStoreId: publicText(128, false).optional(),
  })
  .strict()
  .transform(({ externalStoreId, ...input }) =>
    externalStoreId ? { ...input, externalStoreId } : input,
  );

export const workbenchStoreUpdateSchema = z
  .object({
    name: publicText(120, true),
    region: publicText(64, true),
  })
  .strict();

export const workbenchStoreListFiltersSchema = z
  .object({
    page: z.number().int().min(1).max(Number.MAX_SAFE_INTEGER),
    pageSize: z.number().int().min(1).max(100),
    platform: workbenchStorePlatformSchema.optional(),
    status: workbenchStoreRecordStatusSchema.optional(),
  })
  .strict();

export type WorkbenchStoreCreateInput = z.infer<
  typeof workbenchStoreCreateSchema
>;
export type WorkbenchStoreUpdateInput = z.infer<
  typeof workbenchStoreUpdateSchema
>;
export type WorkbenchStoreListFilters = z.infer<
  typeof workbenchStoreListFiltersSchema
>;

export function hasValidStoreServiceFacts(store: { recordStatus: string; serviceStatus: string | null; serviceStartedAt: string | null; serviceExpiresAt: string | null }) {
  const { recordStatus, serviceStatus, serviceStartedAt: start, serviceExpiresAt: expiry } = store;
  if (recordStatus === "deleting") return serviceStatus === null && start === null && expiry === null;
  if (serviceStatus === "pending_activation") return start === null && expiry === null;
  const orderedPeriod = start !== null && expiry !== null && Date.parse(expiry) > Date.parse(start);
  if (serviceStatus === "active" || serviceStatus === "expired") return orderedPeriod;
  return serviceStatus === "suspended" && ((start === null && expiry === null) || orderedPeriod);
}
