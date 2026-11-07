import { useState } from "react";
import type { PurchaseDate } from "../../api/types";
import { FieldShell } from "./Field";
import styles from "./Field.module.css";

const ADD_TIME = "Add time";
const REMOVE_TIME = "Date only";
const DATE_LENGTH = 10;
const DATE_TIME_LENGTH = 16;

interface DateFieldProps {
  label: string;
  optional?: boolean;
  value: PurchaseDate | null;
  onValueChange: (value: PurchaseDate | null) => void;
  error?: string;
}

/** Date with an optional time. Never defaults to today; empty means unknown. */
export function DateField({
  label,
  optional,
  value,
  onValueChange,
  error,
}: DateFieldProps) {
  const [withTime, setWithTime] = useState(value?.precision === "datetime");
  const update = (text: string, timed: boolean) => {
    if (text === "") {
      onValueChange(null);
      return;
    }
    onValueChange({ value: text, precision: timed ? "datetime" : "date" });
  };
  const toggle = () => {
    const timed = !withTime;
    setWithTime(timed);
    if (!value) {
      return;
    }
    const datePart = value.value.slice(0, DATE_LENGTH);
    update(timed ? `${datePart}T12:00` : datePart, timed);
  };
  return (
    <FieldShell label={label} optional={optional} error={error}>
      {({ id, describedBy, invalid }) => (
        <div className={styles.control} data-invalid={invalid || undefined}>
          <input
            id={id}
            className={styles.input}
            type={withTime ? "datetime-local" : "date"}
            value={
              withTime
                ? (value?.value ?? "").slice(0, DATE_TIME_LENGTH)
                : (value?.value ?? "").slice(0, DATE_LENGTH)
            }
            onChange={(event) => update(event.target.value, withTime)}
            aria-describedby={describedBy}
            aria-invalid={invalid || undefined}
          />
          <button
            type="button"
            className={styles.inlineToggle}
            onClick={toggle}
          >
            {withTime ? REMOVE_TIME : ADD_TIME}
          </button>
        </div>
      )}
    </FieldShell>
  );
}
