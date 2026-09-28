import { expect, it } from "vitest";
import {
  MemberResourceError,
  parseMemberResourceResult,
  resourceInteger,
} from "./member-resources";

it.each(["bad", "1.5", "1e3", "-1", "01", "", "9223372036854775808"])(
  "resource integers reject %s without throwing from safeParse",
  (value) => {
    expect(resourceInteger.safeParse(value).success).toBe(false);
  },
);

const offer = {
  offerId: "offer",
  pricingVersion: "price",
  currency: "CNY",
  unitPriceMinor: "3",
  minQuantity: "1",
  maxQuantity: "100",
};
const quote = {
  organizationId: "org",
  quoteId: "quote",
  quantity: "3",
  unitPriceMinor: "3",
  pricingVersion: "price",
  currency: "CNY",
  expiresAt: "2026-09-29T00:00:00Z",
  amountMinor: "10",
  remainderMinor: "1",
};
const transfer = {
  organizationId: "org",
  memberId: "member",
  resourceType: "data_row",
  operationId: "key",
  position: {
    free: "3",
    reserved: "0",
    consumed: "0",
    version: "1",
    recorded: true,
  },
  unallocated: "7",
  allocated: "3",
  grossCredit: "0",
  debtRepaid: "0",
  netCredit: "0",
  replayed: false,
};
it.each([
  {
    path: "/data-prices",
    method: "GET",
    payload: {
      organizationId: "org",
      offers: [{ ...offer, minQuantity: "bad" }],
    },
  },
  ...["quantity", "unitPriceMinor", "amountMinor", "remainderMinor"].map(
    (field) => ({
      path: "/data-quotes",
      method: "POST",
      payload: { ...quote, [field]: "bad" },
    }),
  ),
  ...["grossCredit", "debtRepaid", "netCredit"].map((field) => ({
    path: "/members/member/transfers",
    method: "POST",
    payload: { ...transfer, [field]: "bad" },
  })),
])(
  "rejects malformed numeric upstream $path through the typed error contract",
  ({ path, method, payload }) => {
    expect(() =>
      parseMemberResourceResult(payload, path, method, "org", "key"),
    ).toThrow(MemberResourceError);
  },
);
