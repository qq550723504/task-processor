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
      submission_state: {
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
      submission_state: {
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
      submission_state: {
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

  it("maps SHEIN online auth freshness blockers to store login guidance", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "shein_online_auth",
            label: "SHEIN 在线登录态",
            message: "SHEIN 提交店铺当前不可用，请先刷新登录态后再提交：store token missing",
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

  it("maps category freshness blockers to category guidance", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "shein_category_template_freshness",
            label: "类目模板新鲜度",
            message: "当前类目模板已发生变化",
          },
        ],
      },
    });

    expect(issues[0]).toMatchObject({
      category: "类目问题",
      title: "类目模板已经变化",
      severity: "blocking",
      actionKey: "category",
    });
  });

  it("maps sale attribute freshness blockers to sale attribute guidance", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "shein_sale_attribute_freshness",
            label: "销售属性模板新鲜度",
            message: "当前销售属性模板已变化",
          },
        ],
      },
    });

    expect(issues[0]).toMatchObject({
      category: "销售属性问题",
      title: "销售属性模板已经变化",
      severity: "blocking",
      actionKey: "sale_attributes",
    });
  });

  it("maps size chart payload blockers without treating them as sale attributes", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        ready: false,
        blocking_items: [
          {
            key: "variants",
            label: "发布载荷结构",
            message:
              "SHEIN publish blocked: missing required size chart attributes: 内侧裤长 (cm), 胸围 (cm)",
          },
        ],
      },
    });

    expect(issues[0]).toMatchObject({
      category: "发布载荷问题",
      title: "尺码表字段需要补齐",
      actionKey: "variants",
    });
  });

  it("maps pod platform blockers to pod guidance", () => {
    const issues = buildSheinCustomerIssues({
      submit_readiness: {
        blocking_items: [
          {
            key: "pod_platform",
            label: "POD 平台处理",
            message: "SDS 平台处理为发布前置，当前不可提交：design template unavailable",
          },
        ],
      },
    });

    expect(issues[0]).toMatchObject({
      category: "图片问题",
      title: "POD 平台结果还未就绪",
      severity: "blocking",
      actionKey: "pod_platform",
    });
  });

  it("maps degraded pod size-image fallback to image guidance", () => {
    const issues = buildSheinCustomerIssues({
      pod_execution: {
        provider: "sds",
        dependency_mode: "optional",
        status: "failed_degraded",
        failure_reason: "size image render unavailable",
      },
    });

    expect(issues[0]).toMatchObject({
      category: "图片问题",
      title: "POD 尺寸图已降级",
      severity: "warning",
      actionKey: "images",
    });
  });

  it("keeps unknown errors as raw other issues", () => {
    const issues = buildSheinCustomerIssues({
      submission_state: {
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
      submission_state: {
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
