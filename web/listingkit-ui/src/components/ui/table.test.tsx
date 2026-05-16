import { render, screen } from "@testing-library/react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

describe("Table", () => {
  it("provides shadcn table structure classes", () => {
    render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>Template</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    expect(screen.getByRole("table")).toHaveClass("w-full");
    expect(screen.getByRole("columnheader", { name: "Name" })).toHaveClass(
      "text-muted-foreground",
    );
    expect(screen.getByRole("cell", { name: "Template" })).toHaveClass("p-4");
  });
});
