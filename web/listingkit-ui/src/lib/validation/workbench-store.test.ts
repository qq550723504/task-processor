import { describe, expect, it } from "vitest";

import {
  workbenchStoreCreateSchema,
  workbenchStoreListFiltersSchema,
  workbenchStoreUpdateSchema,
} from "@/lib/validation/workbench-store";

describe("workbench Store input validation", () => {
  it("trims public create fields and normalizes blank external IDs like omission", () => {
    const omitted = workbenchStoreCreateSchema.parse({
      name: "  North Shop  ",
      platform: "shein",
      region: " SG ",
    });
    const blank = workbenchStoreCreateSchema.parse({
      name: "  North Shop  ",
      platform: "shein",
      region: " SG ",
      externalStoreId: "   ",
    });

    expect(omitted).toEqual({
      name: "North Shop",
      platform: "shein",
      region: "SG",
    });
    expect(blank).toEqual(omitted);
  });

  it("counts Unicode code points instead of UTF-16 code units", () => {
    expect(
      workbenchStoreCreateSchema.safeParse({
        name: "😀".repeat(120),
        platform: "shein",
        region: "🇸🇬".repeat(32),
        externalStoreId: "😀".repeat(128),
      }).success,
    ).toBe(true);
    expect(
      workbenchStoreCreateSchema.safeParse({
        name: "😀".repeat(121),
        platform: "shein",
        region: "SG",
      }).success,
    ).toBe(false);
    expect(
      workbenchStoreCreateSchema.safeParse({
        name: "Shop",
        platform: "shein",
        region: "界".repeat(65),
      }).success,
    ).toBe(false);
    expect(
      workbenchStoreCreateSchema.safeParse({
        name: "Shop",
        platform: "shein",
        region: "SG",
        externalStoreId: "界".repeat(129),
      }).success,
    ).toBe(false);
  });

  it("rejects blank required text and Unicode control characters", () => {
    for (const input of [
      { name: "   ", platform: "shein", region: "SG" },
      { name: "Shop\u0000", platform: "shein", region: "SG" },
      { name: "Shop", platform: "shein", region: "\tSG" },
      {
        name: "Shop",
        platform: "shein",
        region: "SG",
        externalStoreId: "external\u007f",
      },
    ]) {
      expect(workbenchStoreCreateSchema.safeParse(input).success).toBe(false);
    }
  });

  it("accepts only the exact create and update fields", () => {
    const forbidden = [
      "organizationId",
      "tenantId",
      "username",
      "password",
      "credential",
      "connectionRef",
      "role",
      "subject",
      "userId",
    ];
    for (const field of forbidden) {
      expect(
        workbenchStoreCreateSchema.safeParse({
          name: "Shop",
          platform: "shein",
          region: "SG",
          [field]: "secret",
        }).success,
      ).toBe(false);
      expect(
        workbenchStoreUpdateSchema.safeParse({
          name: "Shop",
          region: "SG",
          [field]: "secret",
        }).success,
      ).toBe(false);
    }
    expect(
      workbenchStoreUpdateSchema.parse({ name: " Renamed ", region: " MY " }),
    ).toEqual({ name: "Renamed", region: "MY" });
  });

  it("accepts only SHEIN and the lifecycle list statuses", () => {
    expect(
      workbenchStoreCreateSchema.safeParse({
        name: "Shop",
        platform: "amazon",
        region: "SG",
      }).success,
    ).toBe(false);
    for (const status of [
      "provisioning",
      "active",
      "disabled",
      "deleting",
    ]) {
      expect(
        workbenchStoreListFiltersSchema.safeParse({
          page: 1,
          pageSize: 20,
          platform: "shein",
          status,
        }).success,
      ).toBe(true);
    }
    expect(
      workbenchStoreListFiltersSchema.safeParse({
        page: 1,
        pageSize: 20,
        status: "deleted",
      }).success,
    ).toBe(false);
  });

  it("enforces safe list ranges and rejects unknown selectors", () => {
    expect(
      workbenchStoreListFiltersSchema.parse({
        page: Number.MAX_SAFE_INTEGER,
        pageSize: 100,
      }),
    ).toEqual({ page: Number.MAX_SAFE_INTEGER, pageSize: 100 });
    for (const input of [
      { page: 0, pageSize: 20 },
      { page: 1.5, pageSize: 20 },
      { page: Number.MAX_SAFE_INTEGER + 1, pageSize: 20 },
      { page: 1, pageSize: 0 },
      { page: 1, pageSize: 101 },
      { page: 1, pageSize: 20, organizationId: "org-a" },
      { page: 1, pageSize: 20, arbitrary: "value" },
    ]) {
      expect(workbenchStoreListFiltersSchema.safeParse(input).success).toBe(
        false,
      );
    }
  });
});
