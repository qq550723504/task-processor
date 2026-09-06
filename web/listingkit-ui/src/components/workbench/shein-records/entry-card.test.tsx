import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { SheinRecordsEntry } from "./entry-card";

afterEach(cleanup);

it("links the existing workbench to the local collection when assembled", () => {
  render(<SheinRecordsEntry available />);
  expect(screen.getByRole("heading", { name: "SHEIN 本地资料" })).toBeVisible();
  expect(screen.getByRole("link", { name: /查看本地资料/ })).toHaveAttribute("href", "/workbench/shein-records");
  expect(screen.queryByRole("button", { name: /生成|发布|编辑/ })).not.toBeInTheDocument();
});

it("clearly disables the entry without offering a link when not assembled", () => {
  render(<SheinRecordsEntry available={false} />);
  expect(screen.getByText("暂未启用")).toBeVisible();
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
});
