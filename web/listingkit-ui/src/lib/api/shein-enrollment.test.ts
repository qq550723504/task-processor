import { beforeEach, describe, expect, it, vi } from "vitest";

import { apiRequest } from "@/lib/api/client";
import {
  executeSheinActivityEnrollment,
  getSheinActivityStrategy,
  getSheinActivityCandidates,
  getSheinActivityEnrollmentRunItems,
  getSheinActivityEnrollmentRuns,
  getSheinEnrollmentDashboard,
  getSheinEnrollmentStoreSummary,
  getSheinSDSCostGroups,
  getSheinSourceSDSMetadata,
  getSheinSyncedProducts,
  refreshSheinActivityCandidates,
  resetSheinActivityCandidates,
  reviewSheinActivityCandidate,
  syncSheinSourceSDSProduct,
  triggerSheinStoreSync,
  updateSheinActivityStrategy,
  updateSheinSDSCostGroup,
  updateSheinSyncedProductCost,
} from "@/lib/api/shein-enrollment";

vi.mock("@/lib/api/client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api/client")>(
    "@/lib/api/client",
  );
  return {
    ...actual,
    apiRequest: vi.fn(),
  };
});

const mockedApiRequest = vi.mocked(apiRequest);

describe("shein enrollment api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedApiRequest.mockResolvedValue({});
  });

  it("uses the real backend sync route with an empty default payload", async () => {
    await triggerSheinStoreSync(12);

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/shein-sync/stores/12/sync",
      {
        method: "POST",
        body: {},
      },
    );
  });

  it("routes dashboard and store summary APIs through the listingkit shein sync endpoints", async () => {
    await getSheinEnrollmentDashboard({ activity_type: "PROMOTION" });
    await getSheinEnrollmentStoreSummary(12, { activity_type: "PROMOTION" });

    expect(mockedApiRequest).toHaveBeenNthCalledWith(1, "/shein-sync/dashboard", {
      query: { activity_type: "PROMOTION" },
    });
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      2,
      "/shein-sync/stores/12/summary",
      {
        query: { activity_type: "PROMOTION" },
      },
    );
  });

  it("routes store sync and product APIs through the listingkit shein sync endpoints", async () => {
    await triggerSheinStoreSync(12, { trigger_mode: "manual" });
    await getSheinSyncedProducts(12, {
      skc_name: "dress",
      is_active: true,
      page: 2,
      page_size: 50,
    });
    await updateSheinSyncedProductCost(88, { manual_cost_price: 19.5 });
    await getSheinSDSCostGroups(12, { page: 1, page_size: 100 });
    await getSheinSourceSDSMetadata(12, ["XB0610007001", "XB0608018002"]);
    await syncSheinSourceSDSProduct(12, "XB0603003001");
    await updateSheinSDSCostGroup(12, "style:B3195DA6", {
      group_label: "B3195DA6",
      manual_cost_price: 46.8,
    });

    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      1,
      "/shein-sync/stores/12/sync",
      {
        method: "POST",
        body: { trigger_mode: "manual" },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      2,
      "/shein-sync/stores/12/products",
      {
        query: {
          skc_name: "dress",
          is_active: true,
          page: 2,
          page_size: 50,
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      3,
      "/shein-sync/products/88/cost",
      {
        method: "PATCH",
        body: { manual_cost_price: 19.5 },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      4,
      "/shein-sync/stores/12/sds-cost-groups",
      {
        query: { page: 1, page_size: 100 },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      5,
      "/shein-sync/stores/12/source-sds-metadata",
      {
        query: { source_codes: "XB0610007001,XB0608018002" },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      6,
      "/shein-sync/stores/12/source-sds-products/XB0603003001/sync",
      {
        method: "POST",
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      7,
      "/shein-sync/stores/12/sds-cost-groups/style%3AB3195DA6/cost",
      {
        method: "PATCH",
        body: { group_label: "B3195DA6", manual_cost_price: 46.8 },
      },
    );
  });

  it("routes candidate and enrollment APIs through the listingkit shein sync endpoints", async () => {
    await refreshSheinActivityCandidates(12, { activity_type: "flash_sale" });
    await getSheinActivityCandidates(12, {
      activity_type: "flash_sale",
      activity_key: "flash_sale#12",
      skc_name: "dress",
      candidate_version: "20260605",
      executable_only: true,
      page: 1,
      page_size: 20,
    });
    await reviewSheinActivityCandidate(66, {
      store_id: 12,
      review_status: "approved",
      auto_mode_eligible: true,
      selected_for_run: true,
    });
    await resetSheinActivityCandidates(12, {
      activity_type: "flash_sale",
      eligibility_reason: "missing effective cost price",
    });
    await executeSheinActivityEnrollment(12, {
      activity_type: "flash_sale",
      activity_key: "flash_sale#12",
      trigger_mode: "manual_confirmed",
      candidate_ids: [66, 67],
    });
    await getSheinActivityEnrollmentRuns(12, {
      activity_type: "flash_sale",
      activity_key: "flash_sale#12",
      page: 1,
      page_size: 20,
    });
    await getSheinActivityEnrollmentRunItems(12, 99, {
      page: 1,
      page_size: 50,
    });

    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      1,
      "/shein-sync/stores/12/candidates/refresh",
      {
        method: "POST",
        body: { activity_type: "flash_sale" },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      2,
      "/shein-sync/stores/12/candidates",
      {
        query: {
          activity_type: "flash_sale",
          activity_key: "flash_sale#12",
          skc_name: "dress",
          candidate_version: "20260605",
          executable_only: true,
          page: 1,
          page_size: 20,
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      3,
      "/shein-sync/candidates/66/review",
      {
        method: "PATCH",
        body: {
          store_id: 12,
          review_status: "approved",
          auto_mode_eligible: true,
          selected_for_run: true,
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      4,
      "/shein-sync/stores/12/candidates/reset",
      {
        method: "POST",
        body: {
          activity_type: "flash_sale",
          eligibility_reason: "missing effective cost price",
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      5,
      "/shein-sync/stores/12/enrollments",
      {
        method: "POST",
        body: {
          activity_type: "flash_sale",
          activity_key: "flash_sale#12",
          trigger_mode: "manual_confirmed",
          candidate_ids: [66, 67],
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      6,
      "/shein-sync/stores/12/enrollment-runs",
      {
        query: {
          activity_type: "flash_sale",
          activity_key: "flash_sale#12",
          page: 1,
          page_size: 20,
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      7,
      "/shein-sync/stores/12/enrollment-runs/99/items",
      {
        query: {
          page: 1,
          page_size: 50,
        },
      },
    );
  });

  it("routes activity strategy APIs through store scoped shein sync endpoints", async () => {
    await getSheinActivityStrategy(12, "TIME_LIMITED");
    await updateSheinActivityStrategy(12, {
      activity_type: "TIME_LIMITED",
      activity_price_mode: "DISCOUNT",
      activity_partake_type: "REGULAR",
      activity_discount_rate: 0.18,
      activity_min_profit_rate: 0.15,
      fixed_price_adjustment: 1.2,
    });

    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      1,
      "/shein-sync/stores/12/activity-strategy",
      {
        query: {
          activity_type: "TIME_LIMITED",
        },
      },
    );
    expect(mockedApiRequest).toHaveBeenNthCalledWith(
      2,
      "/shein-sync/stores/12/activity-strategy",
      {
        method: "PATCH",
        body: {
          activity_type: "TIME_LIMITED",
          activity_price_mode: "DISCOUNT",
          activity_partake_type: "REGULAR",
          activity_discount_rate: 0.18,
          activity_min_profit_rate: 0.15,
          fixed_price_adjustment: 1.2,
        },
      },
    );
  });
});
