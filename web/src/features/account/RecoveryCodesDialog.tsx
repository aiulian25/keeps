import { useState } from "react";
import { Copy, Download } from "lucide-react";
import { Button } from "../../components/Button";
import { RECOVERY_CODES_FILENAME, PLAIN_TEXT_TYPE } from "./constants";
import { RECOVERY_COPY } from "./copy";
import { PersistentDialog } from "./PersistentDialog";
import { useCopied } from "./useCopied";
import styles from "./Account.module.css";

function downloadCodes(codes: string[]) {
  const text = [RECOVERY_COPY.fileHeading, "", ...codes, ""].join("\n");
  const url = URL.createObjectURL(new Blob([text], { type: PLAIN_TEXT_TYPE }));
  const link = document.createElement("a");
  link.href = url;
  link.download = RECOVERY_CODES_FILENAME;
  link.click();
  URL.revokeObjectURL(url);
}

/** Shown once; the codes live only in the parent's state until "Done". */
export function RecoveryCodesDialog({
  codes,
  onDone,
}: {
  codes: string[] | null;
  onDone: () => void;
}) {
  const [saved, setSaved] = useState(false);
  const { copied, copy } = useCopied();

  const done = () => {
    setSaved(false);
    onDone();
  };

  return (
    <PersistentDialog
      open={codes !== null}
      locked={!saved}
      title={RECOVERY_COPY.dialogTitle}
      onClose={done}
      footer={
        <Button variant="primary" disabled={!saved} onClick={done}>
          {RECOVERY_COPY.done}
        </Button>
      }
    >
      <p className="muted">{RECOVERY_COPY.dialogText}</p>
      <ol className={styles.codeList} aria-label={RECOVERY_COPY.listLabel}>
        {codes?.map((code) => (
          <li key={code}>
            <code>{code}</code>
          </li>
        ))}
      </ol>
      <div className={styles.inlineActions}>
        <Button
          compact
          icon={<Download aria-hidden="true" />}
          onClick={() => downloadCodes(codes ?? [])}
        >
          {RECOVERY_COPY.download}
        </Button>
        <Button
          compact
          icon={<Copy aria-hidden="true" />}
          onClick={() => copy((codes ?? []).join("\n"))}
        >
          {copied ? RECOVERY_COPY.copied : RECOVERY_COPY.copy}
        </Button>
      </div>
      <label className={styles.checkbox}>
        <input
          type="checkbox"
          checked={saved}
          onChange={(event) => setSaved(event.target.checked)}
        />
        {RECOVERY_COPY.savedCheckbox}
      </label>
    </PersistentDialog>
  );
}
