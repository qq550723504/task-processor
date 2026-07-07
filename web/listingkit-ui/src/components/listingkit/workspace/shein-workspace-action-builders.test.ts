import {
  buildApplyManualSheinSaleAttributesRevision,
  buildConfirmCurrentSheinSaleAttributesRevision,
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

  it("prefers the selected template sale attribute value over stale manual text", () => {
    const revision = buildApplyManualSheinSaleAttributesRevision({
      sheinPreview: {
        editor_context: {
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 27,
              secondary_attribute_id: 87,
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  attributes: { Color: "White" },
                  sku_patches: [
                    {
                      supplier_sku: "NS6104229008",
                      attributes: { Size: "5XL" },
                    },
                  ],
                },
              ],
            },
          },
        },
      } as never,
      primaryOption: {
        attribute_id: 27,
        name: "Color",
        name_en: "Color",
        attribute_value_list: [
          {
            attribute_value_id: 112,
            value: "White",
            value_en: "White",
          },
        ],
      },
      secondaryOption: {
        attribute_id: 87,
        name: "Size",
        name_en: "Size",
        attribute_value_list: [
          {
            attribute_value_id: 4004,
            value: "XXXXL",
            value_en: "XXXXL",
          },
        ],
      },
      skcSelections: {
        "SKC-1": { valueId: 112 },
      },
      skuSelections: {
        NS6104229008: { valueId: 4004, textValue: "5XL" },
      },
    });

    expect(revision?.shein?.skc_patches?.[0]?.sku_patches?.[0]).toEqual({
      supplier_sku: "NS6104229008",
      attributes: { Size: "5XL" },
      sale_attributes: [
        {
          scope: "sku",
          name: "Size",
          value: "XXXXL",
          attribute_id: 87,
          attribute_value_id: 4004,
          matched_by: "manual_review",
        },
      ],
    });
  });

  it("includes current skc patches when confirming current sale attributes", () => {
    const revision = buildConfirmCurrentSheinSaleAttributesRevision({
      editor_context: {
        sale_attributes: {
          current: {
            status: "partial",
            primary_attribute_id: 27,
            secondary_attribute_id: 87,
            skc_attributes: [
              {
                scope: "skc",
                name: "Color",
                value: "white",
                attribute_id: 27,
                attribute_value_id: 739,
              },
            ],
            sku_attributes: [
              {
                scope: "sku",
                name: "Size",
                value: "60×70.8Inch (152×180cm)",
                attribute_id: 87,
                attribute_value_id: 303468379,
              },
            ],
            selection_summary: [
              "主销售属性使用源维度 Color 映射到 Color",
              "次销售属性使用源维度 Size 映射到 Size",
            ],
            primary_source_dimension: "Color",
            secondary_source_dimension: "Size",
            skc_value_assignments: {
              white: {
                scope: "skc",
                name: "Color",
                value: "white",
                attribute_id: 27,
                attribute_value_id: 739,
              },
            },
            sku_value_assignments: {
              "60×70.8inch (152×180cm)": {
                scope: "sku",
                name: "Size",
                value: "60×70.8Inch (152×180cm)",
                attribute_id: 87,
                attribute_value_id: 303468379,
              },
            },
            skc_patches: [
              {
                supplier_code: "SKC-1",
                sale_attribute: {
                  scope: "skc",
                  name: "Color",
                  value: "white",
                  attribute_id: 27,
                  attribute_value_id: 739,
                },
                sku_patches: [
                  {
                    supplier_sku: "SKU-1",
                    sale_attributes: [
                      {
                        scope: "sku",
                        name: "Size",
                        value: "60×70.8Inch (152×180cm)",
                        attribute_id: 87,
                        attribute_value_id: 303468379,
                      },
                    ],
                  },
                ],
              },
            ],
          },
        },
        revision_skeleton: {
          shein: {
            sale_attribute_resolution: {
              status: "partial",
              source: "sale_attribute_templates",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_value_assignments: {
                white: {
                  scope: "skc",
                  name: "Color",
                  value: "white",
                  attribute_id: 27,
                  attribute_value_id: 739,
                },
              },
              sku_value_assignments: {
                "60×70.8inch (152×180cm)": {
                  scope: "sku",
                  name: "Size",
                  value: "60×70.8Inch (152×180cm)",
                  attribute_id: 87,
                  attribute_value_id: 303468379,
                },
              },
            },
          },
        },
      },
    } as never);

    expect(revision).toEqual({
      platform: "shein",
      actor: "workspace",
      reason: "Confirm current SHEIN sale attributes",
      shein: {
        sale_attribute_resolution: {
          status: "resolved",
          source: "manual_review",
          recommend_category_review: false,
          category_review_reason: "",
          primary_attribute_id: 27,
          secondary_attribute_id: 87,
          primary_source_dimension: "Color",
          secondary_source_dimension: "Size",
          skc_attributes: [
            {
              scope: "skc",
              name: "Color",
              value: "white",
              attribute_id: 27,
              attribute_value_id: 739,
            },
          ],
          sku_attributes: [
            {
              scope: "sku",
              name: "Size",
              value: "60×70.8Inch (152×180cm)",
              attribute_id: 87,
              attribute_value_id: 303468379,
            },
          ],
          skc_value_assignments: {
            white: {
              scope: "skc",
              name: "Color",
              value: "white",
              attribute_id: 27,
              attribute_value_id: 739,
            },
          },
          sku_value_assignments: {
            "60×70.8inch (152×180cm)": {
              scope: "sku",
              name: "Size",
              value: "60×70.8Inch (152×180cm)",
              attribute_id: 87,
              attribute_value_id: 303468379,
            },
          },
          selection_summary: [
            "主销售属性使用源维度 Color 映射到 Color",
            "次销售属性使用源维度 Size 映射到 Size",
          ],
          review_notes: [
            "SHEIN 销售属性已按当前主规格和其他规格人工确认。",
          ],
        },
        skc_patches: [
          {
            supplier_code: "SKC-1",
            sale_attribute: {
              scope: "skc",
              name: "Color",
              value: "white",
              attribute_id: 27,
              attribute_value_id: 739,
            },
            sku_patches: [
              {
                supplier_sku: "SKU-1",
                sale_attributes: [
                  {
                    scope: "sku",
                    name: "Size",
                    value: "60×70.8Inch (152×180cm)",
                    attribute_id: 87,
                    attribute_value_id: 303468379,
                  },
                ],
              },
            ],
          },
        ],
      },
    });
  });
});
