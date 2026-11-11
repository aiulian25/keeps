import { useState, type FormEvent } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { itemsApi } from "../../api/endpoints";
import type { Unit, UnitInput } from "../../api/types";
import { Disclosure } from "../../components/Bits";
import { Button, IconButton } from "../../components/Button";
import { FormError, TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { SINGLE_QUANTITY } from "../../constants/limits";
import { fieldErrors, formErrorMessage } from "../account/errors";
import { UNITS_COPY } from "./copy";
import styles from "./ItemDetail.module.css";

const emptyUnit: UnitInput = { label: "", serialNumber: "" };

function unitName(unit: Unit, position: number) {
  return unit.label || UNITS_COPY.unnamed(position);
}

interface UnitFormProps {
  initial: UnitInput;
  onSave: (unit: UnitInput) => Promise<unknown>;
  onCancel: () => void;
}

function UnitForm({ initial, onSave, onCancel }: UnitFormProps) {
  const [unit, setUnit] = useState(initial);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const set = (field: keyof UnitInput, value: string) =>
    setUnit((current) => ({ ...current, [field]: value }));
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onSave(unit);
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
          label={UNITS_COPY.label}
          optional
          value={unit.label}
          onValueChange={(value) => set("label", value)}
          placeholder={UNITS_COPY.labelPlaceholder}
          error={problems.label}
          autoFocus
        />
        <TextField
          label={UNITS_COPY.serialNumber}
          optional
          value={unit.serialNumber}
          onValueChange={(value) => set("serialNumber", value)}
          error={problems.serialNumber}
        />
      </div>
      <div className={styles.entryActions}>
        <Button onClick={onCancel}>{COMMON.cancel}</Button>
        <Button type="submit" variant="primary" busy={busy}>
          {UNITS_COPY.save}
        </Button>
      </div>
    </form>
  );
}

interface UnitIdentifiersProps {
  itemId: number;
  quantity: number;
  units: Unit[];
  onChanged: () => Promise<unknown>;
}

/**
 * Item detail → Individual units: a label and printed serial for each unit
 * of an item bought as several. Shown once there is more than one unit.
 */
export function UnitIdentifiers({
  itemId,
  quantity,
  units,
  onChanged,
}: UnitIdentifiersProps) {
  const [editing, setEditing] = useState<number | "new" | null>(null);
  const [removeError, setRemoveError] = useState("");
  const singleUnit = quantity <= SINGLE_QUANTITY && units.length === 0;
  if (singleUnit) {
    return null;
  }
  const allIdentified = units.length >= quantity;
  const close = () => setEditing(null);
  const save = async (unit: UnitInput, unitId?: number) => {
    if (unitId === undefined) {
      await itemsApi.addUnit(itemId, unit);
    } else {
      await itemsApi.updateUnit(itemId, unitId, unit);
    }
    close();
    await onChanged();
  };
  const remove = async (unit: Unit) => {
    setRemoveError("");
    try {
      await itemsApi.removeUnit(itemId, unit.id);
      await onChanged();
    } catch (caught) {
      setRemoveError(formErrorMessage(caught, []));
    }
  };

  return (
    <Disclosure
      summary={UNITS_COPY.title(units.length, quantity)}
      defaultOpen={units.length > 0}
    >
      <div className={styles.entries}>
        <p className="muted small">{UNITS_COPY.explanation}</p>
        <FormError message={removeError} />
        {units.length > 0 && (
          <ol className={styles.entryList}>
            {units.map((unit, index) => {
              const name = unitName(unit, index + 1);
              if (editing === unit.id) {
                return (
                  <li key={unit.id}>
                    <UnitForm
                      initial={unit}
                      onSave={(next) => save(next, unit.id)}
                      onCancel={close}
                    />
                  </li>
                );
              }
              return (
                <li key={unit.id} className={styles.entry}>
                  <div className={styles.entryText}>
                    <span className={styles.entryName}>{name}</span>
                    {unit.serialNumber && (
                      <code className={styles.entryCode}>
                        {unit.serialNumber}
                      </code>
                    )}
                  </div>
                  <IconButton
                    label={UNITS_COPY.edit(name)}
                    onClick={() => setEditing(unit.id)}
                  >
                    <Pencil aria-hidden="true" />
                  </IconButton>
                  <IconButton
                    label={UNITS_COPY.remove(name)}
                    onClick={() => remove(unit)}
                  >
                    <Trash2 aria-hidden="true" />
                  </IconButton>
                </li>
              );
            })}
          </ol>
        )}
        {editing === "new" && (
          <UnitForm initial={emptyUnit} onSave={save} onCancel={close} />
        )}
        {editing !== "new" && allIdentified && (
          <p className="muted small">{UNITS_COPY.allIdentified}</p>
        )}
        {editing !== "new" && !allIdentified && (
          <div>
            <Button
              compact
              icon={<Plus aria-hidden="true" />}
              onClick={() => setEditing("new")}
            >
              {UNITS_COPY.add}
            </Button>
          </div>
        )}
      </div>
    </Disclosure>
  );
}
