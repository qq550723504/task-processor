import { describe, expect, it } from "vitest";

import { buildSheinCustomerIssues } from "./shein-customer-issues";

describe("buildSheinCustomerIssues", () => {
  it("maps readiness image blockers to customer image issues", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "images",
            label: "Missing square image",
            message: "方形图必须有一个",
          },
        ],
      },
    });

    expect(issues).toHaveLength(1);
    expect(issues[0]).toMatchObject({
      category: "图片问题",
      actionKey: "images",
      severity: "blocking",
    });
  });

  it("maps square image validation note to image action", () => {
    const issues = buildSheinCustomerIssues({
      submission: {
        last_result: {
          validation_notes: ["方形图必须有一个"],
        },
      },
    });

    expect(issues[0]).toMatchObject({
      category: "图片问题",
      title: "缺少方形图",
      actionKey: "images",
    });
  });

  it("maps material required validation note to attributes action", () => {
    const issues = buildSheinCustomerIssues({
      submission: {
        last_result: {
          validation_notes: ["材质: 类型下模板属性为必填项"],
        },
      },
    });

    expect(issues[0]).toMatchObject({
      category: "普通属性问题",
      actionKey: "attributes",
    });
  });

  it("maps primary specification validation note to sale attributes action", () => {
    const issues = buildSheinCustomerIssues({
      submission: {
        last_result: {
          validation_notes: ["[颜色]不可以作为主规格，请重新选择"],
        },
      },
    });

    expect(issues[0]).toMatchObject({
      category: "销售属性问题",
      actionKey: "sale_attributes",
    });
  });

  it("maps SHEIN cookie blockers to store login guidance", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "shein_cookie_unavailable",
            label: "SHEIN 店铺登录",
            message: "SHEIN 店铺 cookie 不可用，当前无法在线获取类目、属性和销售属性模板。",
          },
        ],
      },
    });

    expect(issues[0]).toMatchObject({
      category: "提交接口问题",
      title: "SHEIN 店铺需要重新登录",
      severity: "blocking",
      actionKey: "store_login",
    });
  });

  it("keeps unknown errors as raw other issues", () => {
    const issues = buildSheinCustomerIssues({
      submission: {
        last_error: "SHEIN returned an unexpected vendor error",
      },
    });

    expect(issues[0]).toMatchObject({
      category: "其他问题",
      rawText: "SHEIN returned an unexpected vendor error",
    });
  });

  it("deduplicates equivalent readiness and submission issues", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "attribute_review",
            label: "属性复核",
            message: "普通属性仍需要人工确认",
          },
        ],
      },
      submission: {
        last_result: {
          validation_notes: ["材质: 类型下模板属性为必填项"],
        },
      },
    });

    expect(issues).toHaveLength(1);
    expect(issues[0]).toMatchObject({
      category: "普通属性问题",
      title: "商品属性需要补齐",
      actionKey: "attributes",
    });
  });
});
