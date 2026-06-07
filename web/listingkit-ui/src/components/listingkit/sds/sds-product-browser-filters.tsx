import type { FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import {
  sdsCycleBands,
  sdsWeightBands,
} from "@/lib/sds/product-filters";

type SDSShipmentAreaOption = {
  value: string;
  label: string;
  totalCount?: number;
};

type SDSCategoryOption = {
  id: number;
  name: string;
  count?: number;
};

const selectionQueryReset = {
  productId: undefined,
  variantId: undefined,
  parentProductId: undefined,
  prototypeGroupId: undefined,
  layerId: undefined,
  printWidth: undefined,
  printHeight: undefined,
  templateImageUrl: undefined,
  maskImageUrl: undefined,
  blankDesignUrl: undefined,
  mockupImageUrl: undefined,
  mockupImageUrls: undefined,
  productName: undefined,
  variantLabel: undefined,
};

export function SDSProductBrowserFilters({
  availableCategories,
  availableShipmentAreas,
  categoriesLoading,
  categoryId,
  cycleBand,
  hotSellOnly,
  onSaleOnly,
  queryKeyword,
  shipmentArea,
  shipmentAreasLoading,
  sortValue,
  updateQuery,
  weightBand,
}: {
  availableCategories: SDSCategoryOption[];
  availableShipmentAreas: SDSShipmentAreaOption[];
  categoriesLoading?: boolean;
  categoryId?: number;
  cycleBand: string;
  hotSellOnly: boolean;
  onSaleOnly: boolean;
  queryKeyword: string;
  shipmentArea: string;
  shipmentAreasLoading?: boolean;
  sortValue: string;
  updateQuery: (next: Record<string, string | undefined>) => void;
  weightBand: string;
}) {
  function applySearch(keywordValue: string) {
    updateQuery({
      keyword: keywordValue.trim() || undefined,
      page: "1",
    });
  }

  return (
    <>
      <form
        className="grid gap-3 rounded-lg border border-border bg-muted p-3 shadow-sm md:grid-cols-2 xl:grid-cols-6"
        onSubmit={(event: FormEvent<HTMLFormElement>) => {
          event.preventDefault();
          const formData = new FormData(event.currentTarget);
          applySearch(String(formData.get("keyword") ?? ""));
        }}
      >
        <Select
          className="h-11 min-w-0"
          disabled={shipmentAreasLoading || availableShipmentAreas.length === 0}
          defaultValue={shipmentArea}
          key={shipmentArea}
          name="shipmentArea"
          onChange={(event) =>
            updateQuery({
              shipmentArea: event.target.value,
              page: "1",
              categoryId: undefined,
              ...selectionQueryReset,
            })
          }
        >
          {availableShipmentAreas.map((area) => (
            <option key={area.value} value={area.value}>
              {area.label} ({area.totalCount})
            </option>
          ))}
        </Select>
        <Select
          className="h-11 min-w-0"
          disabled={categoriesLoading}
          key={`${shipmentArea}:${categoryId ?? 0}`}
          name="categoryId"
          onChange={(event) =>
            updateQuery({
              categoryId: event.target.value || undefined,
              page: "1",
              ...selectionQueryReset,
            })
          }
          defaultValue={categoryId ? String(categoryId) : ""}
        >
          <option value="">全部分类</option>
          {availableCategories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name} ({category.count})
            </option>
          ))}
        </Select>
        <Select
          className="h-11 min-w-0"
          defaultValue={sortValue}
          key={`sort:${sortValue || "default"}`}
          name="sort"
          onChange={(event) =>
            updateQuery({
              sort: event.target.value || undefined,
              page: "1",
            })
          }
        >
          <option value="">默认排序</option>
          <option value="min_price:asc">价格从低到高</option>
          <option value="min_price:desc">价格从高到低</option>
        </Select>
        <Select
          className="h-11 min-w-0"
          defaultValue={weightBand}
          key={`weight:${weightBand || "all"}`}
          name="weightBand"
          onChange={(event) =>
            updateQuery({
              weightBand: event.target.value || undefined,
              page: "1",
            })
          }
        >
          {sdsWeightBands.map((band) => (
            <option key={band.value || "all"} value={band.value}>
              {band.label}
            </option>
          ))}
        </Select>
        <Select
          className="h-11 min-w-0"
          defaultValue={cycleBand}
          key={`cycle:${cycleBand || "all"}`}
          name="cycleBand"
          onChange={(event) =>
            updateQuery({
              cycleBand: event.target.value || undefined,
              page: "1",
            })
          }
        >
          {sdsCycleBands.map((band) => (
            <option key={band.value || "all"} value={band.value}>
              {band.label}
            </option>
          ))}
        </Select>
        <Input
          className="h-11 min-w-0 md:col-span-2 xl:col-span-5"
          defaultValue={queryKeyword}
          key={queryKeyword}
          name="keyword"
          placeholder="按商品名或 SKU 搜索"
        />
        <div>
          <Button type="submit">搜索</Button>
        </div>
      </form>

      <div className="flex flex-wrap gap-2">
        <Button
          variant={onSaleOnly ? "default" : "outline"}
          onClick={() =>
            updateQuery({
              onSaleStatus: onSaleOnly ? undefined : "2",
              page: "1",
            })
          }
          type="button"
        >
          只看在售
        </Button>
        <Button
          variant={hotSellOnly ? "default" : "outline"}
          onClick={() =>
            updateQuery({
              hotSellStatus: hotSellOnly ? undefined : "1",
              page: "1",
            })
          }
          type="button"
        >
          只看热卖
        </Button>
      </div>
    </>
  );
}
