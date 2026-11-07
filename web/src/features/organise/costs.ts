import type { CostSummary } from "../../api/types";
import { formatMoney } from "../items/itemFormat";

const CURRENCY_SEPARATOR = " + ";

/** "£1247.50 + €80"; empty when nothing in the group has a price. */
export function formatCosts(summary: CostSummary | undefined) {
  if (!summary) {
    return "";
  }
  return Object.entries(summary.totals)
    .sort(([first], [second]) => first.localeCompare(second))
    .map(([currency, amount]) => formatMoney({ amount, currency }))
    .join(CURRENCY_SEPARATOR);
}
