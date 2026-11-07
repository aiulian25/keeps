import type { Money } from "../../api/types";
import { FieldShell } from "./Field";
import styles from "./Field.module.css";

const CURRENCY_SYMBOLS: Record<string, string> = {
  GBP: "£",
  EUR: "€",
  USD: "$",
  JPY: "¥",
};

export function currencySymbol(currency: string) {
  return CURRENCY_SYMBOLS[currency] ?? currency;
}

interface MoneyFieldProps {
  label: string;
  optional?: boolean;
  value: Money | null;
  defaultCurrency: string;
  onValueChange: (value: Money | null) => void;
  placeholder?: string;
  error?: string;
  hint?: string;
}

/** Amount plus currency. Empty means unknown, never zero. */
export function MoneyField({
  label,
  optional,
  value,
  defaultCurrency,
  onValueChange,
  placeholder,
  error,
  hint,
}: MoneyFieldProps) {
  const currency = value?.currency || defaultCurrency;
  const update = (amount: string, nextCurrency: string) => {
    if (amount.trim() === "") {
      onValueChange(null);
      return;
    }
    onValueChange({ amount, currency: nextCurrency });
  };
  return (
    <FieldShell label={label} optional={optional} error={error} hint={hint}>
      {({ id, describedBy, invalid }) => (
        <div className={styles.control} data-invalid={invalid || undefined}>
          <span className={styles.prefix} aria-hidden="true">
            {currencySymbol(currency)}
          </span>
          <input
            id={id}
            className={styles.input}
            inputMode="decimal"
            autoComplete="off"
            value={value?.amount ?? ""}
            placeholder={placeholder}
            onChange={(event) => update(event.target.value, currency)}
            aria-describedby={describedBy}
            aria-invalid={invalid || undefined}
          />
          <input
            className={`${styles.input} ${styles.currencyInput}`}
            aria-label={`${label} currency`}
            maxLength={3}
            value={currency}
            onChange={(event) =>
              update(value?.amount ?? "", event.target.value.toUpperCase())
            }
          />
        </div>
      )}
    </FieldShell>
  );
}
