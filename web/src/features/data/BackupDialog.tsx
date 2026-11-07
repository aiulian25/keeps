import { useId, useState, type FormEvent } from "react";
import type { PreparedFile } from "../../api/types";
import { TriangleAlert } from "lucide-react";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, PasswordField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { BACKUP_PASSPHRASE_MINIMUM } from "../../constants/limits";
import { fieldErrors, formErrorMessage } from "../account/errors";
import { BACKUP_COPY } from "./copy";
import { MissingFilesList } from "./MissingFilesList";
import styles from "./Data.module.css";

interface BackupDialogProps {
  open: boolean;
  onPrepare: (passphrase: string) => Promise<PreparedFile>;
  onDownload: (prepared: PreparedFile) => void;
  onClose: () => void;
}

function validate(passphrase: string, confirmation: string) {
  if ([...passphrase].length < BACKUP_PASSPHRASE_MINIMUM) {
    return { passphrase: BACKUP_COPY.tooShort(BACKUP_PASSPHRASE_MINIMUM) };
  }
  if (passphrase !== confirmation) {
    return { confirmation: BACKUP_COPY.mismatch };
  }
  return {};
}

export function BackupDialog({
  open,
  onPrepare,
  onDownload,
  onClose,
}: BackupDialogProps) {
  const formId = useId();
  const [passphrase, setPassphrase] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [problems, setProblems] = useState<{
    passphrase?: string;
    confirmation?: string;
  }>({});
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [incomplete, setIncomplete] = useState<PreparedFile | null>(null);

  const close = () => {
    setIncomplete(null);
    setPassphrase("");
    setConfirmation("");
    setProblems({});
    setError(null);
    onClose();
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const found = validate(passphrase, confirmation);
    setProblems(found);
    setError(null);
    if (Object.keys(found).length > 0) {
      return;
    }
    setBusy(true);
    try {
      const prepared = await onPrepare(passphrase);
      if (!prepared.complete) {
        setIncomplete(prepared);
        return;
      }
      onDownload(prepared);
      close();
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  const downloadAnyway = () => {
    if (incomplete) {
      onDownload(incomplete);
    }
    close();
  };

  if (incomplete) {
    return (
      <Dialog
        open={open}
        title={BACKUP_COPY.incompleteTitle}
        onClose={close}
        footer={
          <>
            <Button onClick={close}>{COMMON.cancel}</Button>
            <Button variant="primary" onClick={downloadAnyway}>
              {BACKUP_COPY.downloadAnyway}
            </Button>
          </>
        }
      >
        <div className="stack">
          <p className={styles.warning} role="alert">
            <TriangleAlert aria-hidden="true" />
            {BACKUP_COPY.incompleteText(incomplete.missingFiles.length)}
          </p>
          <MissingFilesList files={incomplete.missingFiles} />
        </div>
      </Dialog>
    );
  }

  return (
    <Dialog
      open={open}
      title={BACKUP_COPY.title}
      onClose={close}
      footer={
        <>
          <Button onClick={close}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={BACKUP_COPY.preparing}
          >
            {BACKUP_COPY.submit}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} className="stack" noValidate>
        <p className="muted">{BACKUP_COPY.intro}</p>
        <p className={styles.warning}>
          <TriangleAlert aria-hidden="true" />
          {BACKUP_COPY.warning}
        </p>
        <FormError message={formErrorMessage(error, ["passphrase"])} />
        <PasswordField
          label={BACKUP_COPY.passphrase}
          autoComplete="new-password"
          hint={BACKUP_COPY.passphraseHint(BACKUP_PASSPHRASE_MINIMUM)}
          value={passphrase}
          onValueChange={setPassphrase}
          error={problems.passphrase ?? fieldErrors(error).passphrase}
          required
          autoFocus
        />
        <PasswordField
          label={BACKUP_COPY.confirm}
          autoComplete="new-password"
          value={confirmation}
          onValueChange={setConfirmation}
          error={problems.confirmation}
          required
        />
      </form>
    </Dialog>
  );
}
