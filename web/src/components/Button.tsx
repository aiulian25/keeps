import type { ButtonHTMLAttributes, ReactNode } from "react";
import { Link } from "react-router";
import styles from "./Button.module.css";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "outline-accent";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  icon?: ReactNode;
  iconAfter?: ReactNode;
  busy?: boolean;
  busyLabel?: string;
  wide?: boolean;
  compact?: boolean;
}

export function buttonClass(
  variant: Variant = "secondary",
  wide = false,
  compact = false,
) {
  return [
    styles.button,
    styles[variant],
    wide ? styles.wide : "",
    compact ? styles.compact : "",
  ]
    .filter(Boolean)
    .join(" ");
}

export function Button({
  variant = "secondary",
  icon,
  iconAfter,
  busy = false,
  busyLabel,
  wide = false,
  compact = false,
  children,
  className,
  disabled,
  type = "button",
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={[buttonClass(variant, wide, compact), className]
        .filter(Boolean)
        .join(" ")}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      {...rest}
    >
      {icon}
      <span>{busy && busyLabel ? busyLabel : children}</span>
      {iconAfter}
    </button>
  );
}

interface ButtonLinkProps {
  to: string;
  variant?: Variant;
  icon?: ReactNode;
  iconAfter?: ReactNode;
  wide?: boolean;
  compact?: boolean;
  children: ReactNode;
  state?: unknown;
}

export function ButtonLink({
  to,
  variant = "secondary",
  icon,
  iconAfter,
  wide = false,
  compact = false,
  children,
  state,
}: ButtonLinkProps) {
  return (
    <Link to={to} state={state} className={buttonClass(variant, wide, compact)}>
      {icon}
      <span>{children}</span>
      {iconAfter}
    </Link>
  );
}

export function IconButton({
  label,
  children,
  className,
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      className={[styles.iconButton, className].filter(Boolean).join(" ")}
      {...rest}
    >
      {children}
    </button>
  );
}
