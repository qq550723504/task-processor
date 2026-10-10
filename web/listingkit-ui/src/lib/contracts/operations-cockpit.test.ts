import { expect, it } from "vitest";
import { moneyCents, moneyText, ratioText } from "./operations-cockpit";
it("keeps cents and derived rational display exact without float rounding", () => {
 expect(moneyCents("100.25")).toBe(10025);expect(moneyCents("0")).toBe(0);expect(moneyCents("1.005")).toBeNull();expect(moneyCents("10000000000.01")).toBeNull();
 expect(moneyText(9007199254740991)).toBe("¥90,071,992,547,409.91");
 expect(moneyText(-101)).toBe("-¥1.01");expect(ratioText({ numerator: "-1", denominator: "3" })).toBe("-33.3%");
});
