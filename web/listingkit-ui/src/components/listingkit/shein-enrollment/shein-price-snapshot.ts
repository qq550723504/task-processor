"use client";

const SHEIN_PRICE_LOCALE = "en-US";

export type SheinSKUPriceSnapshot = {
  skuCode: string;
  price: string;
  currency: string;
};

export function formatSheinPriceSnapshot(value?: string | null) {
  const text = value?.trim();
  if (!text) {
    return "-";
  }

  try {
    const parsed = JSON.parse(text) as {
      sale_price?: unknown;
      currency?: unknown;
    };
    if (typeof parsed === "object" && parsed !== null) {
      const amount = Number(parsed.sale_price);
      const currency = typeof parsed.currency === "string" ? parsed.currency : "";
      if (Number.isFinite(amount) && currency) {
        return formatCurrencyAmount(currency, amount, text);
      }
    }
  } catch {
    // Fall back to plain text parsing below.
  }

  const match = text.match(/^([A-Z]{3})\s+(-?\d+(?:\.\d+)?)$/);
  if (!match) {
    return text;
  }

  const [, currency, amountText] = match;
  const amount = Number(amountText);
  if (!Number.isFinite(amount)) {
    return text;
  }

  return formatCurrencyAmount(currency, amount, text);
}

export function formatSheinCurrencyAmount(currency: string, amount: number) {
  const normalizedCurrency = currency.trim();
  if (!normalizedCurrency || !Number.isFinite(amount)) {
    return "-";
  }
  return formatCurrencyAmount(
    normalizedCurrency,
    amount,
    `${normalizedCurrency} ${amount.toFixed(2)}`,
  );
}

function formatCurrencyAmount(currency: string, amount: number, fallback: string) {
  try {
    return new Intl.NumberFormat(SHEIN_PRICE_LOCALE, {
      style: "currency",
      currency,
      currencyDisplay: "narrowSymbol",
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount);
  } catch {
    return fallback;
  }
}

export function getSheinSKUPriceSnapshots(
  value?: string | null,
): SheinSKUPriceSnapshot[] {
  const text = value?.trim();
  if (!text) {
    return [];
  }

  try {
    const parsed = JSON.parse(text) as {
      currency?: unknown;
      sku_prices?: unknown;
    };
    if (typeof parsed !== "object" || parsed === null || !Array.isArray(parsed.sku_prices)) {
      return [];
    }

    const snapshotCurrency = typeof parsed.currency === "string" ? parsed.currency : "";
    return parsed.sku_prices.flatMap((entry) => {
      if (typeof entry !== "object" || entry === null) {
        return [];
      }

      const skuPrice = entry as {
        sku_code?: unknown;
        sale_price?: unknown;
        currency?: unknown;
      };
      const skuCode = typeof skuPrice.sku_code === "string" ? skuPrice.sku_code.trim() : "";
      const amount = Number(skuPrice.sale_price);
      const currency =
        typeof skuPrice.currency === "string" && skuPrice.currency.trim()
          ? skuPrice.currency.trim()
          : snapshotCurrency;
      if (!skuCode || !Number.isFinite(amount)) {
        return [];
      }

      return [
        {
          skuCode,
          currency,
          price: currency
            ? formatCurrencyAmount(currency, amount, `${currency} ${amount.toFixed(2)}`)
            : amount.toFixed(2),
        },
      ];
    });
  } catch {
    return [];
  }
}

export function getSheinSKUSupplyPriceSnapshots(
  value?: string | null,
): SheinSKUPriceSnapshot[] {
  const text = value?.trim();
  if (!text) {
    return [];
  }

  try {
    const parsed = JSON.parse(text) as {
      sku_supply_prices?: unknown;
    };
    if (
      typeof parsed !== "object" ||
      parsed === null ||
      !Array.isArray(parsed.sku_supply_prices)
    ) {
      return [];
    }

    return parsed.sku_supply_prices.flatMap((entry) => {
      if (typeof entry !== "object" || entry === null) {
        return [];
      }

      const skuSupplyPrice = entry as {
        sku_code?: unknown;
        supply_price?: unknown;
        currency?: unknown;
      };
      const skuCode =
        typeof skuSupplyPrice.sku_code === "string" ? skuSupplyPrice.sku_code.trim() : "";
      const amount = Number(skuSupplyPrice.supply_price);
      const currency =
        typeof skuSupplyPrice.currency === "string" ? skuSupplyPrice.currency.trim() : "";
      if (!skuCode || !Number.isFinite(amount) || amount <= 0) {
        return [];
      }

      return [
        {
          skuCode,
          currency,
          price: currency
            ? formatCurrencyAmount(currency, amount, `${currency} ${amount.toFixed(2)}`)
            : amount.toFixed(2),
        },
      ];
    });
  } catch {
    return [];
  }
}
