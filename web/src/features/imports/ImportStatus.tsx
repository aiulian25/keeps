import { Link } from "react-router";
import { ROUTES } from "../../constants/routes";
import { BUTTON_COPY } from "../add/copy";
import { Link2Off, Pencil, Upload } from "lucide-react";
import { Button } from "../../components/Button";
import { GENERIC_PROBLEM, IMPORT_COPY, PROBLEM_COPY } from "./copy";
import styles from "./Import.module.css";

interface ProgressProps {
  host: string;
  onCancel: () => void;
  onAddManually: () => void;
}

/** UX-06: a plain status line, no percentage, and two ways out. */
export function ImportProgress({
  host,
  onCancel,
  onAddManually,
}: ProgressProps) {
  const status = host
    ? IMPORT_COPY.gettingDetails(host)
    : IMPORT_COPY.gettingDetailsUnknownHost;
  return (
    <div className={styles.centered}>
      <p className={styles.status} role="status" aria-live="polite">
        {status}
      </p>
      <div className={styles.pairButtons}>
        <Button onClick={onCancel}>{IMPORT_COPY.cancel}</Button>
        <Button icon={<Pencil aria-hidden="true" />} onClick={onAddManually}>
          {IMPORT_COPY.addManuallyInstead}
        </Button>
      </div>
    </div>
  );
}

interface ProblemProps {
  code: string;
  reason?: string;
  onAddManually: () => void;
  onUploadReceipt: () => void;
  onRetry?: () => void;
  retryLabel?: string;
}

/** Panel 06: what happened, in two lines, and one click to carry on. */
export function ImportProblem({
  code,
  reason,
  onAddManually,
  onUploadReceipt,
  onRetry,
  retryLabel = IMPORT_COPY.tryAgain,
}: ProblemProps) {
  const text = PROBLEM_COPY[code] ?? GENERIC_PROBLEM;
  return (
    <div className={styles.centered}>
      <span className={styles.problemIcon}>
        <Link2Off aria-hidden="true" />
      </span>
      <h1>{text.title}</h1>
      <p className={styles.reason}>{reason || text.reason}</p>
      <div className={styles.pairButtons}>
        <Button icon={<Pencil aria-hidden="true" />} onClick={onAddManually}>
          {IMPORT_COPY.addManually}
        </Button>
        <Button icon={<Upload aria-hidden="true" />} onClick={onUploadReceipt}>
          {IMPORT_COPY.uploadReceipt}
        </Button>
      </div>
      {onRetry && (
        <button type="button" className={styles.textButton} onClick={onRetry}>
          {retryLabel}
        </button>
      )}
      {BUTTON_HELPS.has(code) && (
        <Link to={ROUTES.addButton} className={styles.textButton}>
          {BUTTON_COPY.shopBlocks}
        </Link>
      )}
    </div>
  );
}

/** Problems the Add to Keeps button can get around, because the page loads fine in the person's own browser. */
const BUTTON_HELPS = new Set([
  "blocked",
  "timeout",
  "no_details",
  "fetch_failed",
]);
