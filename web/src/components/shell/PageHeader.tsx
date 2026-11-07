import type { ReactNode } from "react";
import { ArrowLeft } from "lucide-react";
import { COMMON } from "../../constants/copy";
import { useGoBack } from "../../hooks/useGoBack";
import styles from "./Shell.module.css";

/** Goes back to the previous page; `fallback` is used when there is none. */
function BackArrow({ fallback, label }: { fallback: string; label: string }) {
  const goBack = useGoBack(fallback);
  return (
    <a
      href={fallback}
      className={styles.backLink}
      aria-label={label}
      onClick={(event) => {
        event.preventDefault();
        goBack();
      }}
    >
      <ArrowLeft aria-hidden="true" />
    </a>
  );
}

interface PageHeaderProps {
  title: ReactNode;
  /** Where the back arrow goes if there is no previous page in Keeps. */
  backTo?: string;
  backLabel?: string;
  aside?: ReactNode;
  icon?: ReactNode;
  subtitle?: ReactNode;
}

export function PageHeader({
  title,
  backTo,
  backLabel = COMMON.back,
  aside,
  icon,
  subtitle,
}: PageHeaderProps) {
  return (
    <header className={styles.pageHeader}>
      <div className={styles.pageTitle}>
        {backTo && <BackArrow fallback={backTo} label={backLabel} />}
        {icon}
        <div>
          <h1>{title}</h1>
          {subtitle && <p className="muted">{subtitle}</p>}
        </div>
      </div>
      {aside && <div className={styles.pageAside}>{aside}</div>}
    </header>
  );
}
