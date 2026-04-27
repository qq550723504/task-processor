export function formatSDSPrice(value?: number | null) {
  if (!value) {
    return "-";
  }
  return `¥${value.toFixed(2)}`;
}

