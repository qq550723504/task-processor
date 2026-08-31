import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StoreTable } from "@/components/workbench/stores/store-table";

const STORE = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "华东旗舰店",
  platform: "shein" as const,
  region: "CN",
  externalStoreId: "",
  lifecycleStatus: "active" as const,
  connectionStatus: "disconnected" as const,
  version: 1,
  createdAt: "2026-08-31T00:00:00Z",
  updatedAt: "2026-08-31T01:02:03Z",
};

describe("StoreTable", () => {
  it("renders the exact directory projection without inferring connection state", () => {
    render(
      <StoreTable
        stores={[
          STORE,
          {
            ...STORE,
            id: "22222222-2222-4222-8222-222222222222",
            name: "欧洲店",
            lifecycleStatus: "provisioning",
            connectionStatus: "connected",
            externalStoreId: "eu-1",
          },
          {
            ...STORE,
            id: "33333333-3333-4333-8333-333333333333",
            name: "北美店",
            lifecycleStatus: "disabled",
            connectionStatus: "expired",
          },
          {
            ...STORE,
            id: "44444444-4444-4444-8444-444444444444",
            name: "测试店",
            lifecycleStatus: "deleting",
            connectionStatus: "unavailable",
            updatedAt: "invalid",
          },
        ]}
      />,
    );

    const table = screen.getByRole("table", { name: "我的店铺列表" });
    expect(within(table).getAllByRole("columnheader").map((header) => header.textContent)).toEqual([
      "店铺名称",
      "平台",
      "区域",
      "外部店铺 ID",
      "店铺状态",
      "连接状态",
      "更新时间",
      "操作",
    ]);
    expect(within(table).getAllByText("未设置")).toHaveLength(3);
    expect(within(table).getAllByText("SHEIN")).toHaveLength(4);
    expect(within(table).getByText("已启用")).toBeInTheDocument();
    expect(within(table).getByText("开通中")).toBeInTheDocument();
    expect(within(table).getByText("已停用")).toBeInTheDocument();
    expect(within(table).getByText("删除中")).toBeInTheDocument();
    expect(within(table).getByText("未连接")).toBeInTheDocument();
    expect(within(table).getByText("已连接")).toBeInTheDocument();
    expect(within(table).getByText("授权已过期")).toBeInTheDocument();
    expect(within(table).getByText("暂时无法检查")).toBeInTheDocument();
    expect(within(table).getByRole("link", { name: "查看华东旗舰店" })).toHaveAttribute(
      "href",
      "/workbench/stores/11111111-1111-4111-8111-111111111111",
    );
    expect(within(table).getAllByText("2026-08-31 01:02 UTC")[0]?.closest("time")).toHaveAttribute(
      "dateTime",
      "2026-08-31T01:02:03Z",
    );
    expect(within(table).getByText("—").closest("time")).toHaveAttribute(
      "dateTime",
      "",
    );
  });
});
