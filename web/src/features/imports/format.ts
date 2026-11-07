import type { Money } from "../../api/types";

const fetchedDateFormat = new Intl.DateTimeFormat(undefined, {
  day: "numeric",
  month: "short",
});

/** "6 Nov" style; empty when the server sent no time. */
export function formatFetchedDate(timestamp?: string) {
  if (!timestamp) {
    return "";
  }
  const date = new Date(timestamp);
  return Number.isNaN(date.getTime()) ? "" : fetchedDateFormat.format(date);
}

export function formatMoney(money: Money) {
  try {
    return new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: money.currency,
    }).format(Number(money.amount));
  } catch {
    return `${money.amount} ${money.currency}`;
  }
}
