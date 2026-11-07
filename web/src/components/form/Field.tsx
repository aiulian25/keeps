import {
  useId,
  useState,
  type InputHTMLAttributes,
  type ReactNode,
  type TextareaHTMLAttributes,
} from "react";
import { Eye, EyeOff } from "lucide-react";
import { COMMON } from "../../constants/copy";
import styles from "./Field.module.css";

interface FieldShellProps {
  label: string;
  optional?: boolean;
  required?: boolean;
  hint?: string;
  error?: string;
  children: (props: {
    id: string;
    describedBy?: string;
    invalid: boolean;
  }) => ReactNode;
  className?: string;
}

/** Label, control, hint and inline error, wired for screen readers. */
export function FieldShell({
  label,
  optional,
  required,
  hint,
  error,
  children,
  className,
}: FieldShellProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;
  return (
    <div className={[styles.field, className].filter(Boolean).join(" ")}>
      <label htmlFor={id} className={styles.label}>
        {label}
        {required && <span aria-hidden="true"> *</span>}
        {optional && (
          <span className={styles.optional}> {COMMON.optional}</span>
        )}
      </label>
      {children({ id, describedBy, invalid: Boolean(error) })}
      {hint && !error && (
        <p id={hintId} className={styles.hint}>
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className={styles.error}>
          {error}
        </p>
      )}
    </div>
  );
}

type InputProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "onChange" | "prefix"
> & {
  label: string;
  optional?: boolean;
  hint?: string;
  error?: string;
  value: string;
  onValueChange: (value: string) => void;
  prefix?: ReactNode;
  fieldClassName?: string;
};

export function TextField({
  label,
  optional,
  hint,
  error,
  value,
  onValueChange,
  prefix,
  fieldClassName,
  required,
  ...rest
}: InputProps) {
  return (
    <FieldShell
      label={label}
      optional={optional}
      required={required}
      hint={hint}
      error={error}
      className={fieldClassName}
    >
      {({ id, describedBy, invalid }) => (
        <div className={styles.control} data-invalid={invalid || undefined}>
          {prefix && <span className={styles.prefix}>{prefix}</span>}
          <input
            id={id}
            className={styles.input}
            value={value}
            onChange={(event) => onValueChange(event.target.value)}
            aria-describedby={describedBy}
            aria-invalid={invalid || undefined}
            required={required}
            {...rest}
          />
        </div>
      )}
    </FieldShell>
  );
}

export function PasswordField({
  label,
  hint,
  error,
  value,
  onValueChange,
  fieldClassName,
  ...rest
}: Omit<InputProps, "prefix">) {
  const [visible, setVisible] = useState(false);
  const RevealIcon = visible ? EyeOff : Eye;
  return (
    <FieldShell
      label={label}
      hint={hint}
      error={error}
      className={fieldClassName}
    >
      {({ id, describedBy, invalid }) => (
        <div className={styles.control} data-invalid={invalid || undefined}>
          <input
            id={id}
            className={styles.input}
            type={visible ? "text" : "password"}
            value={value}
            onChange={(event) => onValueChange(event.target.value)}
            aria-describedby={describedBy}
            aria-invalid={invalid || undefined}
            {...rest}
          />
          <button
            type="button"
            className={styles.reveal}
            onClick={() => setVisible(!visible)}
            aria-label={visible ? COMMON.hidePassword : COMMON.showPassword}
            aria-pressed={visible}
          >
            <RevealIcon aria-hidden="true" />
          </button>
        </div>
      )}
    </FieldShell>
  );
}

type TextAreaProps = Omit<
  TextareaHTMLAttributes<HTMLTextAreaElement>,
  "onChange"
> & {
  label: string;
  optional?: boolean;
  hint?: string;
  error?: string;
  value: string;
  onValueChange: (value: string) => void;
};

export function TextArea({
  label,
  optional,
  hint,
  error,
  value,
  onValueChange,
  ...rest
}: TextAreaProps) {
  return (
    <FieldShell label={label} optional={optional} hint={hint} error={error}>
      {({ id, describedBy, invalid }) => (
        <textarea
          id={id}
          className={styles.textarea}
          value={value}
          onChange={(event) => onValueChange(event.target.value)}
          aria-describedby={describedBy}
          aria-invalid={invalid || undefined}
          data-invalid={invalid || undefined}
          {...rest}
        />
      )}
    </FieldShell>
  );
}

export function FormError({ message }: { message?: string }) {
  if (!message) {
    return null;
  }
  return (
    <p role="alert" className={styles.formError}>
      {message}
    </p>
  );
}
