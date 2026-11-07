import type { Money, PurchaseDate } from "../../api/types";
import { currencySymbol } from "../../components/form/MoneyField";

const WHOLE_AMOUNT = /\.0+$/;
const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/;
const MONTH_OFFSET = 1;

const dateFormat = new Intl.DateTimeFormat(undefined, {
  day: "numeric",
  month: "short",
  year: "numeric",
});
const dateTimeFormat = new Intl.DateTimeFormat(undefined, {
  day: "numeric",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});
const dayMonthFormat = new Intl.DateTimeFormat(undefined, {
  day: "numeric",
  month: "short",
});

/** Places a comma every three digits in the whole part (text only, no rounding). */
const THOUSANDS_BOUNDARY = /\B(?=(\d{3})+(?!\d))/g;

/** "£79", "£79.50" or "£1,247.50"; the amount string from the API is kept exact. */
export function formatMoney(money: Money) {
  const [whole, fraction] = money.amount.replace(WHOLE_AMOUNT, "").split(".");
  const grouped = whole.replace(THOUSANDS_BOUNDARY, ",");
  return `${currencySymbol(money.currency)}${fraction === undefined ? grouped : `${grouped}.${fraction}`}`;
}

/** Date-only values are calendar days, so they are built in local time to avoid a UTC shift. */
function parseDate(value: string) {
  const dateOnly = DATE_ONLY.exec(value);
  if (!dateOnly) {
    return new Date(value);
  }
  const [, year, month, day] = dateOnly;
  return new Date(Number(year), Number(month) - MONTH_OFFSET, Number(day));
}

/** "12 Mar 2024", plus the time when one was recorded. Unknown dates are never guessed. */
export function formatPurchaseDate(date: PurchaseDate) {
  const parsed = parseDate(date.value);
  if (Number.isNaN(parsed.getTime())) {
    return date.value;
  }
  if (date.precision === "datetime") {
    return dateTimeFormat.format(parsed);
  }
  return dateFormat.format(parsed);
}

export function formatDay(value: string) {
  return dateFormat.format(parseDate(value));
}

export function formatDayMonth(value: string) {
  return dayMonthFormat.format(parseDate(value));
}
