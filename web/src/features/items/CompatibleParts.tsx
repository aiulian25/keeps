import { useState, type FormEvent } from "react";
import { ExternalLink, Pencil, Plus, Trash2 } from "lucide-react";
import { itemsApi } from "../../api/endpoints";
import type { Part, PartInput } from "../../api/types";
import { Disclosure } from "../../components/Bits";
import { Button, IconButton } from "../../components/Button";
import { FormError, TextArea, TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { fieldErrors, formErrorMessage } from "../account/errors";
import { PARTS_COPY } from "./copy";
import styles from "./ItemDetail.module.css";

const emptyPart: PartInput = {
  name: "",
  partNumber: "",
  note: "",
  sourceUrl: "",
};

interface PartFormProps {
  initial: PartInput;
  onSave: (part: PartInput) => Promise<unknown>;
  onCancel: () => void;
}

function PartForm({ initial, onSave, onCancel }: PartFormProps) {
  const [part, setPart] = useState(initial);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const set = (field: keyof PartInput, value: string) =>
    setPart((current) => ({ ...current, [field]: value }));
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onSave(part);
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };
  const problems = fieldErrors(error);
  return (
    <form className={styles.entryForm} onSubmit={submit} noValidate>
      <FormError message={formErrorMessage(error, Object.keys(problems))} />
      <div className={styles.entryFormRow}>
        <TextField
          label={PARTS_COPY.name}
          value={part.name}
          onValueChange={(value) => set("name", value)}
          placeholder={PARTS_COPY.namePlaceholder}
          error={problems.name}
          autoFocus
        />
        <TextField
          label={PARTS_COPY.partNumber}
          optional
          value={part.partNumber}
          onValueChange={(value) => set("partNumber", value)}
          error={problems.partNumber}
        />
      </div>
      <TextArea
        label={PARTS_COPY.note}
        optional
        value={part.note}
        onValueChange={(value) => set("note", value)}
        placeholder={PARTS_COPY.notePlaceholder}
        error={problems.note}
      />
      <TextField
        label={PARTS_COPY.sourceUrl}
        optional
        type="url"
        value={part.sourceUrl}
        onValueChange={(value) => set("sourceUrl", value)}
        hint={PARTS_COPY.sourceHint}
        error={problems.sourceUrl}
      />
      <div className={styles.entryActions}>
        <Button onClick={onCancel}>{COMMON.cancel}</Button>
        <Button type="submit" variant="primary" busy={busy}>
          {PARTS_COPY.save}
        </Button>
      </div>
    </form>
  );
}

interface CompatiblePartsProps {
  itemId: number;
  parts: Part[];
  onChanged: () => Promise<unknown>;
}

/** Item detail → Parts and consumables: what fits, as the owner noted it. */
export function CompatibleParts({
  itemId,
  parts,
  onChanged,
}: CompatiblePartsProps) {
  const [editing, setEditing] = useState<number | "new" | null>(null);
  const [removeError, setRemoveError] = useState("");
  const close = () => setEditing(null);
  const save = async (part: PartInput, partId?: number) => {
    if (partId === undefined) {
      await itemsApi.addPart(itemId, part);
    } else {
      await itemsApi.updatePart(itemId, partId, part);
    }
    close();
    await onChanged();
  };
  const remove = async (part: Part) => {
    setRemoveError("");
    try {
      await itemsApi.removePart(itemId, part.id);
      await onChanged();
    } catch (caught) {
      setRemoveError(formErrorMessage(caught, []));
    }
  };

  return (
    <Disclosure
      summary={PARTS_COPY.title(parts.length)}
      defaultOpen={parts.length > 0}
    >
      <div className={styles.entries}>
        <p className="muted small">{PARTS_COPY.disclaimer}</p>
        <FormError message={removeError} />
        {parts.length > 0 && (
          <ul className={styles.entryList}>
            {parts.map((part) =>
              editing === part.id ? (
                <li key={part.id}>
                  <PartForm
                    initial={part}
                    onSave={(next) => save(next, part.id)}
                    onCancel={close}
                  />
                </li>
              ) : (
                <li key={part.id} className={styles.entry}>
                  <div className={styles.entryText}>
                    <span className={styles.entryName}>{part.name}</span>
                    {part.partNumber && (
                      <code className={styles.entryCode}>
                        {part.partNumber}
                      </code>
                    )}
                    {part.note && <p className="muted small">{part.note}</p>}
                    {part.sourceUrl && (
                      <a
                        className={styles.entrySource}
                        href={part.sourceUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {PARTS_COPY.source}
                        <ExternalLink aria-hidden="true" />
                      </a>
                    )}
                  </div>
                  <IconButton
                    label={PARTS_COPY.edit(part.name)}
                    onClick={() => setEditing(part.id)}
                  >
                    <Pencil aria-hidden="true" />
                  </IconButton>
                  <IconButton
                    label={PARTS_COPY.remove(part.name)}
                    onClick={() => remove(part)}
                  >
                    <Trash2 aria-hidden="true" />
                  </IconButton>
                </li>
              ),
            )}
          </ul>
        )}
        {editing === "new" ? (
          <PartForm initial={emptyPart} onSave={save} onCancel={close} />
        ) : (
          <div>
            <Button
              compact
              icon={<Plus aria-hidden="true" />}
              onClick={() => setEditing("new")}
            >
              {PARTS_COPY.add}
            </Button>
          </div>
        )}
      </div>
    </Disclosure>
  );
}
