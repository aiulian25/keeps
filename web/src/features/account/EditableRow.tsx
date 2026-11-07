import { useState, type FormEvent, type ReactNode } from "react";
import { Button } from "../../components/Button";
import { TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { ACCOUNT_COPY } from "./copy";
import { errorMessage } from "./errors";
import { SettingsRow } from "./SettingsRow";
import styles from "./Account.module.css";

interface EditableRowProps {
  icon: ReactNode;
  label: string;
  value: string;
  hint?: string;
  maxLength?: number;
  normalise?: (value: string) => string;
  onSave: (value: string) => Promise<void>;
}

/** A settings row whose value is edited in place: Edit → field, Save, Cancel. */
export function EditableRow({
  icon,
  label,
  value,
  hint,
  maxLength,
  normalise = (text) => text.trim(),
  onSave,
}: EditableRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const startEditing = () => {
    setDraft(value);
    setError(null);
    setEditing(true);
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onSave(normalise(draft));
      setEditing(false);
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  if (!editing) {
    return (
      <SettingsRow
        icon={icon}
        label={label}
        value={value || <span className="muted">{ACCOUNT_COPY.notSet}</span>}
        action={
          <Button
            compact
            onClick={startEditing}
            aria-label={ACCOUNT_COPY.editLabel(label)}
          >
            {COMMON.edit}
          </Button>
        }
      />
    );
  }

  return (
    <SettingsRow icon={icon} label={label}>
      <form className={styles.inlineForm} onSubmit={save}>
        <TextField
          label={label}
          fieldClassName={styles.inlineField}
          value={draft}
          onValueChange={setDraft}
          hint={hint}
          error={errorMessage(error)}
          maxLength={maxLength}
          autoFocus
          required
        />
        <div className={styles.inlineActions}>
          <Button
            type="submit"
            variant="primary"
            compact
            busy={busy}
            busyLabel={COMMON.saving}
          >
            {COMMON.save}
          </Button>
          <Button compact onClick={() => setEditing(false)}>
            {COMMON.cancel}
          </Button>
        </div>
      </form>
    </SettingsRow>
  );
}
