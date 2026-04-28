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
      title: "缺少方形图",
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
});
