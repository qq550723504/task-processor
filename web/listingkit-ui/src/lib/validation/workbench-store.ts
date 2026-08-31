import { z } from "zod";

const workbenchStorePlatformSchema = z.literal("shein");
const workbenchStoreLifecycleStatusSchema = z.enum([
  "provisioning",
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
    status: workbenchStoreLifecycleStatusSchema.optional(),
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
