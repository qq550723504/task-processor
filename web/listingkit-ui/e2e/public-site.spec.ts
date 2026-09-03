import { expect, test } from "@playwright/test";

test("public homepage exposes the primary workbench entry", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "让智能，成为电商经营的默认能力",
  );
  await expect(page.getByRole("link", { name: "进入系统" })).toHaveAttribute(
    "href",
    "/login?returnTo=%2Fworkbench",
  );
  await expect(page.getByRole("link", { name: /进入硕米 OS/ })).toHaveAttribute(
    "href",
    "/login?returnTo=%2Fworkbench",
  );
});
