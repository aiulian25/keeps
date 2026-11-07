import { useId, useState, type FormEvent } from "react";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { errorMessage } from "../account/errors";
import { EXPORT_COPY } from "./copy";
import type { ExportFormat } from "./useDataActions";
import styles from "./Data.module.css";

const FORMAT_OPTIONS: {
  format: ExportFormat;
  label: string;
  detail: string;
}[] = [
  { format: "csv", label: EXPORT_COPY.csv, detail: EXPORT_COPY.csvDetail },
  {
    format: "archive",
    label: EXPORT_COPY.archive,
    detail: EXPORT_COPY.archiveDetail,
  },
];

interface ExportDialogProps {
  open: boolean;
  onExport: (format: ExportFormat) => Promise<void>;
  onClose: () => void;
}

export function ExportDialog({ open, onExport, onClose }: ExportDialogProps) {
  const formId = useId();
  const [format, setFormat] = useState<ExportFormat>("csv");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onExport(format);
      onClose();
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      title={EXPORT_COPY.title}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={EXPORT_COPY.preparing}
          >
            {EXPORT_COPY.download}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} className="stack">
        <FormError message={errorMessage(error)} />
        <fieldset className={styles.choices}>
          <legend className="visually-hidden">{EXPORT_COPY.legend}</legend>
          {FORMAT_OPTIONS.map((option) => (
            <label key={option.format} className={styles.choice}>
              <input
                type="radio"
                name="format"
                value={option.format}
                checked={format === option.format}
                onChange={() => setFormat(option.format)}
              />
              <span className={styles.choiceText}>
                {option.label}
                <span className={styles.choiceDetail}>{option.detail}</span>
              </span>
            </label>
          ))}
        </fieldset>
      </form>
    </Dialog>
  );
}
