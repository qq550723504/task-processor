import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import WorkbenchPage from "../page";
import SheinRecordsPage from "./page";

vi.mock("next/server", () => ({ connection: vi.fn() }));
// Even a configured backend cannot approve a product entry (#328 / #331).
vi.mock("@/lib/server/shein-records-availability", () => ({ isSheinRecordsAvailable: () => true }));
vi.mock("next/navigation", () => ({ notFound: () => { throw new Error("NEXT_HTTP_ERROR_FALLBACK;404"); } }));
vi.mock("@/components/workbench/shein-records/record-list-page", () => ({ SheinRecordListPage: () => <p>collection mounted</p> }));

afterEach(cleanup);

it("does not advertise the withdrawn platform-specific workbench entry", async () => {
  render(await WorkbenchPage());
  expect(screen.getByRole("heading", { name: "经营全局，一屏掌握" })).toBeVisible();
  expect(screen.queryByText("SHEIN 本地资料")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "查看本地资料" })).not.toBeInTheDocument();
});

it("blocks direct collection navigation even with a configured backend", async () => {
  await expect((async () => SheinRecordsPage())()).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
});
