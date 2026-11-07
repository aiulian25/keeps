import type { SelectHTMLAttributes } from "react";
import { FieldShell } from "./Field";
import styles from "./Field.module.css";

interface SelectFieldProps extends Omit<
  SelectHTMLAttributes<HTMLSelectElement>,
  "onChange"
> {
  label: string;
  optional?: boolean;
  error?: string;
  value: string;
  options: { value: string; label: string }[];
  onValueChange: (value: string) => void;
}

/** A native select styled like the other fields. */
export function SelectField({
  label,
  optional,
  error,
  value,
  options,
  onValueChange,
  ...rest
}: SelectFieldProps) {
  return (
    <FieldShell label={label} optional={optional} error={error}>
      {({ id, describedBy, invalid }) => (
        <div className={styles.control} data-invalid={invalid || undefined}>
          <select
            id={id}
            className={`${styles.input} ${styles.select}`}
            value={value}
            onChange={(event) => onValueChange(event.target.value)}
            aria-describedby={describedBy}
            aria-invalid={invalid || undefined}
            {...rest}
          >
            {options.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      )}
    </FieldShell>
  );
}
