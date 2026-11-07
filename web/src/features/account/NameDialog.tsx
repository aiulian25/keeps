import { useId, useState, type FormEvent } from "react";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { errorMessage, fieldErrors } from "./errors";

interface NameDialogProps {
  open: boolean;
  title: string;
  label: string;
  initialName: string;
  submitLabel: string;
  busyLabel?: string;
  hint?: string;
  onSubmit: (name: string) => Promise<void>;
  onClose: () => void;
  /** Overrides the message for errors that aren't from the API. */
  describeError?: (error: unknown) => string;
}

/** One-field dialog for naming or renaming something. */
export function NameDialog({
  open,
  title,
  label,
  initialName,
  submitLabel,
  busyLabel,
  hint,
  onSubmit,
  onClose,
  describeError = errorMessage,
}: NameDialogProps) {
  const formId = useId();
  const [name, setName] = useState(initialName);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [openedWith, setOpenedWith] = useState<string | null>(null);

  // Reset the field each time the dialog opens for a (possibly different) thing.
  const openKey = open ? initialName : null;
  if (openKey !== openedWith) {
    setOpenedWith(openKey);
    setName(initialName);
    setError(null);
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onSubmit(name.trim());
      onClose();
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  const nameError = fieldErrors(error).name;
  return (
    <Dialog
      open={open}
      title={title}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={busyLabel}
          >
            {submitLabel}
          </Button>
        </>
      }
    >
      <form id={formId} className="stack" onSubmit={submit}>
        <FormError message={nameError ? "" : describeError(error)} />
        <TextField
          label={label}
          hint={hint}
          value={name}
          onValueChange={setName}
          error={nameError}
          required
          autoFocus
        />
      </form>
    </Dialog>
  );
}
