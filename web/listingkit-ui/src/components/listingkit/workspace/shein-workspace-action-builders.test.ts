import {
  buildRefreshCurrentSheinCategoryRevision,
  buildRegenerateSheinAttributesRevision,
  buildRegenerateSheinSaleAttributesRevision,
} from "@/components/listingkit/workspace/shein-workspace-action-builders";

describe("shein workspace action builders", () => {
  it("builds a dedicated revision for refreshing the current category", () => {
    const revision = buildRefreshCurrentSheinCategoryRevision({
      category_id: 10489,
      editor_context: {
        category: {
          current: {
            category_id: 10489,
            category_id_list: [2866, 4396, 4425, 10489],
            product_type_id: 7190,
            top_category_id: 2866,
            category_path: ["运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"],
          },
        },
      },
    } as never);

    expect(revision).toEqual({
      platform: "shein",
      actor: "workspace",
      reason: "Refresh SHEIN category",
      shein: {
        category_resolution: {
          category_id: 10489,
          category_id_list: [2866, 4396, 4425, 10489],
          product_type_id: 7190,
          top_category_id: 2866,
          matched_path: ["运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"],
          source: "manual_refresh",
          status: "resolved",
        },
        sale_attribute_resolution: {
          recommend_category_review: false,
          category_review_reason: "",
        },
      },
    });
  });

  it("builds a dedicated revision for regenerating attributes", () => {
    const revision = buildRegenerateSheinAttributesRevision({
      category_id: 10489,
      editor_context: {
        category: {
          current: {
            category_id: 10489,
            category_id_list: [2866, 4396, 4425, 10489],
            product_type_id: 7190,
            top_category_id: 2866,
            category_path: ["运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"],
            source: "manual_search",
          },
        },
        attributes: {
          current: {
            status: "partial",
          },
        },
      },
    } as never);

    expect(revision).toEqual({
      platform: "shein",
      actor: "workspace",
      reason: "Regenerate SHEIN attributes",
      shein: {
        regenerate_attributes: true,
        category_resolution: {
          category_id: 10489,
          category_id_list: [2866, 4396, 4425, 10489],
          product_type_id: 7190,
          top_category_id: 2866,
          matched_path: ["运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"],
          source: "manual_search",
          status: "resolved",
        },
      },
    });
  });

  it("builds a dedicated revision for regenerating sale attributes", () => {
    const revision = buildRegenerateSheinSaleAttributesRevision({
      category_id: 10489,
      editor_context: {
        category: {
          current: {
            category_id: 10489,
            category_id_list: [2866, 4396, 4425, 10489],
            product_type_id: 7190,
            top_category_id: 2866,
            category_path: ["运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"],
            source: "manual_search",
          },
        },
        sale_attributes: {
          current: {
            status: "partial",
            primary_attribute_id: 1001466,
          },
        },
      },
    } as never);

    expect(revision).toEqual({
      platform: "shein",
      actor: "workspace",
      reason: "Regenerate SHEIN sale attributes",
      shein: {
        regenerate_sale_attributes: true,
        category_resolution: {
          category_id: 10489,
          category_id_list: [2866, 4396, 4425, 10489],
          product_type_id: 7190,
          top_category_id: 2866,
          matched_path: ["运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"],
          source: "manual_search",
          status: "resolved",
        },
      },
    });
  });
});
