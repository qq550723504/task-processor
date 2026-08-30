import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  DataTable,
  type DataTableColumnDef,
} from "@/components/ui/data-table";

type Product = { id: string; name: string; sales: number };

const columns: DataTableColumnDef<Product>[] = [
  { accessorKey: "name", header: "商品" },
  { accessorKey: "sales", header: "销量" },
];

describe("DataTable", () => {
  it("sorts rows through an accessible column button", async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        ariaLabel="商品数据"
        columns={columns}
        data={[
          { id: "2", name: "Zulu", sales: 2 },
          { id: "1", name: "Alpha", sales: 10 },
        ]}
        getRowId={(row) => row.id}
      />,
    );

    await user.click(screen.getByRole("button", { name: "商品" }));
    const rows = screen.getAllByRole("row").slice(1);
    expect(within(rows[0]).getByText("Alpha")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Zulu")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "商品" })).toHaveAttribute(
      "aria-sort",
      "ascending",
    );
  });

  it("renders one semantic empty row", () => {
    render(
      <DataTable
        ariaLabel="商品数据"
        columns={columns}
        data={[]}
        emptyMessage="暂无商品"
      />,
    );

    expect(screen.getByRole("table", { name: "商品数据" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "暂无商品" })).toHaveAttribute(
      "colspan",
      "2",
    );
  });

  it("spans every leaf column in an empty grouped table", () => {
    const groupedColumns: DataTableColumnDef<Product>[] = [
      {
        header: "商品信息",
        columns: [
          { accessorKey: "name", header: "商品" },
          { accessorKey: "sales", header: "销量" },
        ],
      },
      {
        header: "标识信息",
        columns: [{ accessorKey: "id", header: "ID" }],
      },
    ];

    render(
      <DataTable
        ariaLabel="分组商品数据"
        columns={groupedColumns}
        data={[]}
        emptyMessage="暂无分组商品"
      />,
    );

    expect(screen.getByRole("cell", { name: "暂无分组商品" })).toHaveAttribute(
      "colspan",
      "3",
    );
    expect(
      screen.getByRole("columnheader", { name: "商品信息" }),
    ).toHaveAttribute("colspan", "2");
    expect(
      screen.getByRole("columnheader", { name: "标识信息" }),
    ).toHaveAttribute("colspan", "1");
  });
});
