import { expect, test } from "@playwright/test";

test("public homepage exposes the primary workbench entry", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "硕米智能引擎新一代AI电商智能操作系统",
  );
  await expect(page.getByRole("link", { name: "进入硕米", exact: true })).toHaveAttribute(
    "href",
    "/login?returnTo=%2Fworkbench",
  );
  await expect(page.getByRole("link", { name: "了解平台能力", exact: true })).toHaveAttribute(
    "href",
    "#agents",
  );
});
