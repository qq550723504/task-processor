import { expect, test } from "@playwright/test";

test("public homepage exposes the primary workbench entry", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1 })).toContainText("硕米智能引擎");
  await expect(page.getByRole("link", { name: "进入硕米" })).toHaveAttribute(
    "href",
    "/login?returnTo=%2Flisting-kits%2Fhome",
  );
});
