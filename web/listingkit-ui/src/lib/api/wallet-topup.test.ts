import { expect, it } from "vitest";
import {
  parseTopUpCheckout,
  yuanToMinor,
  parseTopUpOptions,
} from "./wallet-topup";

it("parses exact cents without rounding or scientific notation", () => {
  expect(yuanToMinor("100.01")).toBe("10001");
  expect(yuanToMinor("92233720368547758.07")).toBe("9223372036854775807");
  for (const value of [
    "1e2",
    "0",
    "-1",
    "1.001",
    "01",
    "+1",
    "92233720368547758.08",
  ])
    expect(yuanToMinor(value)).toBeNull();
});
it("keeps checkout provider, kind, and trusted destination together", () => {
  const base = {
    organization_id: "org-a",
    order_id: "order-a",
    attempt_id: "attempt-a",
    expires_at: "2026-09-27T12:00:00Z",
  };
  expect(
    parseTopUpCheckout({
      ...base,
      provider: "WECHAT_PAY",
      kind: "QR_CODE",
      payload: "weixin://wxpay/bizpayurl?pr=fixture",
    }),
  ).not.toBeNull();
  for (const payload of [
    "javascript:alert(1)",
    "https://evil.example",
    "https://openapi.alipay.com.evil.example/gateway.do",
    "https://user@openapi.alipay.com/gateway.do",
  ])
    expect(
      parseTopUpCheckout({
        ...base,
        provider: "ALIPAY",
        kind: "REDIRECT",
        payload,
      }),
    ).toBeNull();
  expect(
    parseTopUpCheckout({
      ...base,
      provider: "ALIPAY",
      kind: "QR_CODE",
      payload: "weixin://wxpay/bizpayurl?pr=fixture",
    }),
  ).toBeNull();
});
it("does not invent configured amounts when payments are disabled", () => {
  expect(
    parseTopUpOptions({
      organization_id: "org-a",
      currency: "CNY",
      min_minor: "0",
      max_minor: "0",
      quick_amounts_minor: [],
      channels: [
        {
          provider: "ALIPAY",
          product: "PAGE_PAY",
          available: false,
          reason: "PAYMENT_NOT_CONFIGURED",
        },
        {
          provider: "WECHAT_PAY",
          product: "NATIVE",
          available: false,
          reason: "PAYMENT_NOT_CONFIGURED",
        },
      ],
    }),
  ).not.toBeNull();
});
