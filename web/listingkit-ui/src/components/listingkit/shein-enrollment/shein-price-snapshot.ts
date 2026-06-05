"use client";

const SHEIN_PRICE_LOCALE = "en-US";

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
