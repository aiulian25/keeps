import type { ReactNode } from "react";
import { COMMON } from "../../constants/copy";
import styles from "./Account.module.css";

interface SettingsRowProps {
  icon: ReactNode;
  label: string;
  optional?: boolean;
  description?: ReactNode;
  value?: ReactNode;
  action?: ReactNode;
  children?: ReactNode;
}

/** One row of a settings card: icon, label and description, value, action. */
export function SettingsRow({
  icon,
  label,
  optional = false,
  description,
  value,
  action,
  children,
}: SettingsRowProps) {
  return (
    <li className={styles.row}>
      <span className={styles.rowIcon} aria-hidden="true">
        {icon}
      </span>
      <div className={styles.rowText}>
        <span className={styles.rowLabel}>
          {label}
          {optional && (
            <span className={styles.optional}> {COMMON.optional}</span>
          )}
        </span>
        {description && (
          <span className={styles.rowDescription}>{description}</span>
        )}
        {children}
      </div>
      {value !== undefined && <span className={styles.rowValue}>{value}</span>}
      {action && <div className={styles.rowAction}>{action}</div>}
    </li>
  );
}

export function SettingsCard({ children }: { children: ReactNode }) {
  return <ul className={styles.card}>{children}</ul>;
}

interface SettingsSectionProps {
  title: string;
  hint?: string;
  icon?: ReactNode;
  children: ReactNode;
}

export function SettingsSection({
  title,
  hint,
  icon,
  children,
}: SettingsSectionProps) {
  return (
    <section className={styles.section}>
      <header className={styles.sectionHeader}>
        {icon}
        <div>
          <h2>{title}</h2>
          {hint && <p className="muted small">{hint}</p>}
        </div>
      </header>
      {children}
    </section>
  );
}
