import { useId, useState, type FormEvent, type ReactNode } from "react";
import { ApiError } from "../../api/client";
import type { MissingFile, RestoreResult } from "../../api/types";
import { TriangleAlert } from "lucide-react";
import { Button } from "../../components/Button";
import {
  FieldShell,
  FormError,
  PasswordField,
} from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { fieldErrors, formErrorMessage } from "../account/errors";
import { formatDateTime } from "../account/format";
import { PersistentDialog } from "../account/PersistentDialog";
import { BACKUP_FILE_ACCEPT, PERCENT } from "./constants";
import { RESTORE_COPY } from "./copy";
import { MissingFilesList } from "./MissingFilesList";
import styles from "./Data.module.css";

type Step = "choose" | "confirm" | "incomplete" | "working" | "done";

const INCOMPLETE_CODE = "backup_incomplete";

function missingFilesIn(error: unknown): MissingFile[] | null {
  const isIncomplete =
    error instanceof ApiError && error.code === INCOMPLETE_CODE;
  if (!isIncomplete) {
    return null;
  }
  const details = error.details as { missingFiles?: MissingFile[] } | undefined;
  return details?.missingFiles ?? [];
}

interface RestoreDialogProps {
  open: boolean;
  onRestore: (
    file: File,
    passphrase: string,
    acceptIncomplete: boolean,
    onProgress: (fraction: number) => void,
  ) => Promise<RestoreResult>;
  onFinished: () => void;
  onClose: () => void;
}

function stageState(uploaded: boolean, isUploadStage: boolean) {
  if (isUploadStage) {
    return uploaded ? "done" : "active";
  }
  return uploaded ? "active" : "pending";
}

/**
 * Choose file and passphrase → confirm the replacement → upload with
 * progress → result. The server validates everything before swapping, so a
 * failure leaves current data untouched and returns to the first step.
 */
export function RestoreDialog({
  open,
  onRestore,
  onFinished,
  onClose,
}: RestoreDialogProps) {
  const formId = useId();
  const [step, setStep] = useState<Step>("choose");
  const [file, setFile] = useState<File | null>(null);
  const [passphrase, setPassphrase] = useState("");
  const [fileProblem, setFileProblem] = useState("");
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<RestoreResult | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [missing, setMissing] = useState<MissingFile[]>([]);

  const close = () => {
    setStep("choose");
    setMissing([]);
    setFile(null);
    setPassphrase("");
    setFileProblem("");
    setError(null);
    onClose();
  };

  const review = (event: FormEvent) => {
    event.preventDefault();
    if (!file) {
      setFileProblem(RESTORE_COPY.chooseFile);
      return;
    }
    setFileProblem("");
    setStep("confirm");
  };

  const restore = async (acceptIncomplete: boolean) => {
    if (!file) {
      return;
    }
    setError(null);
    setProgress(0);
    setStep("working");
    try {
      setResult(
        await onRestore(file, passphrase, acceptIncomplete, setProgress),
      );
      setStep("done");
    } catch (caught) {
      const missingFiles = missingFilesIn(caught);
      if (missingFiles) {
        setMissing(missingFiles);
        setStep("incomplete");
        return;
      }
      setError(caught);
      setStep("choose");
    }
  };

  const uploaded = progress >= 1;
  const stages = [
    { label: RESTORE_COPY.stageUpload, state: stageState(uploaded, true) },
    { label: RESTORE_COPY.stageCheck, state: stageState(uploaded, false) },
    { label: RESTORE_COPY.stageRestore, state: stageState(uploaded, false) },
  ];
  const statusText = uploaded
    ? RESTORE_COPY.checking
    : RESTORE_COPY.uploading(Math.round(progress * PERCENT));

  const footers: Record<Step, ReactNode> = {
    choose: (
      <>
        <Button onClick={close}>{COMMON.cancel}</Button>
        <Button type="submit" form={formId} variant="primary">
          {RESTORE_COPY.continue}
        </Button>
      </>
    ),
    confirm: (
      <>
        <Button onClick={() => setStep("choose")}>{RESTORE_COPY.back}</Button>
        <Button variant="danger" onClick={() => restore(false)}>
          {RESTORE_COPY.replace}
        </Button>
      </>
    ),
    incomplete: (
      <>
        <Button onClick={close}>{COMMON.cancel}</Button>
        <Button variant="danger" onClick={() => restore(true)}>
          {RESTORE_COPY.restoreAnyway}
        </Button>
      </>
    ),
    working: null,
    done: (
      <Button variant="primary" onClick={onFinished}>
        {RESTORE_COPY.signInAgain}
      </Button>
    ),
  };

  const titles: Record<Step, string> = {
    choose: RESTORE_COPY.title,
    confirm: RESTORE_COPY.confirmTitle,
    incomplete: RESTORE_COPY.incompleteTitle,
    working: RESTORE_COPY.title,
    done: RESTORE_COPY.doneTitle,
  };

  return (
    <PersistentDialog
      open={open}
      locked={step === "working" || step === "done"}
      title={titles[step]}
      onClose={close}
      footer={footers[step]}
    >
      {step === "choose" && (
        <form id={formId} className="stack" onSubmit={review} noValidate>
          <p className="muted">{RESTORE_COPY.intro}</p>
          <FormError message={formErrorMessage(error, ["passphrase"])} />
          <FieldShell
            label={RESTORE_COPY.file}
            hint={
              file
                ? RESTORE_COPY.selectedFile(file.name)
                : RESTORE_COPY.fileHint
            }
            error={fileProblem}
          >
            {({ id, describedBy, invalid }) => (
              <input
                id={id}
                type="file"
                accept={BACKUP_FILE_ACCEPT}
                className={styles.fileInput}
                aria-describedby={describedBy}
                aria-invalid={invalid || undefined}
                onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              />
            )}
          </FieldShell>
          <PasswordField
            label={RESTORE_COPY.passphrase}
            autoComplete="off"
            value={passphrase}
            onValueChange={setPassphrase}
            error={fieldErrors(error).passphrase}
            required
          />
        </form>
      )}
      {step === "confirm" && (
        <p>{RESTORE_COPY.confirmText(file?.name ?? "")}</p>
      )}
      {step === "incomplete" && (
        <div className="stack">
          <p className={styles.warning} role="alert">
            <TriangleAlert aria-hidden="true" />
            {RESTORE_COPY.incompleteText(missing.length)}
          </p>
          <MissingFilesList files={missing} />
        </div>
      )}
      {step === "working" && (
        <>
          <ol className={styles.stages} aria-label={RESTORE_COPY.stagesLabel}>
            {stages.map((stage) => (
              <li
                key={stage.label}
                data-state={stage.state}
                aria-current={stage.state === "active" ? "step" : undefined}
              >
                {stage.label}
              </li>
            ))}
          </ol>
          <progress
            className={styles.progress}
            max={1}
            value={uploaded ? undefined : progress}
            aria-label={statusText}
          />
          <p role="status" aria-live="polite">
            {statusText}
          </p>
          <p className="muted small">{RESTORE_COPY.stayHere}</p>
        </>
      )}
      {step === "done" && result && (
        <div className="stack">
          <p role="status" aria-live="polite">
            {RESTORE_COPY.doneText(
              formatDateTime(result.createdAt),
              result.files,
            )}
          </p>
          {result.missingFiles.length > 0 && (
            <>
              <p className="muted">
                {RESTORE_COPY.doneMissing(result.missingFiles.length)}
              </p>
              <MissingFilesList files={result.missingFiles} />
            </>
          )}
        </div>
      )}
    </PersistentDialog>
  );
}
