import { useEffect, useState, type FormEvent } from "react";
import { presetsApi } from "../../api/endpoints";
import type { ItemInput, Preset, PresetInput } from "../../api/types";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { fieldErrors, formErrorMessage } from "../account/errors";
import { conditionLabel, ITEM_COPY } from "../items/copy";
import { PRESET_COPY } from "./copy";
import { applyPreset, hasPresetDetails, presetFromInput } from "./presets";
import styles from "./Add.module.css";

const FIELD_LABELS: Record<keyof PresetInput, string> = {
  name: PRESET_COPY.name,
  room: ITEM_COPY.room,
  category: ITEM_COPY.category,
  tags: ITEM_COPY.tags,
  collections: ITEM_COPY.collections,
  condition: ITEM_COPY.condition,
  notes: ITEM_COPY.notes,
};

function describeValue(preset: Preset, field: keyof PresetInput) {
  const value = preset[field];
  if (Array.isArray(value)) {
    return value.join(", ");
  }
  if (field === "condition") {
    return conditionLabel(preset.condition);
  }
  return value;
}

interface CapturePresetDialogProps {
  open: boolean;
  input: ItemInput;
  onApply: (next: ItemInput) => void;
  onClose: () => void;
}

/**
 * Manual add → More details → Use a preset: pick saved starting details,
 * see exactly what they will fill, or save the current details as a preset.
 */
export function CapturePresetDialog({
  open,
  input,
  onApply,
  onClose,
}: CapturePresetDialogProps) {
  const [presets, setPresets] = useState<Preset[] | null>(null);
  const [chosen, setChosen] = useState<number | null>(null);
  const [name, setName] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) {
      return;
    }
    let current = true;
    presetsApi
      .list()
      .then((loaded) => current && setPresets(loaded))
      .catch((caught: unknown) => current && setError(caught));
    return () => {
      current = false;
    };
  }, [open]);

  const preset = presets?.find((item) => item.id === chosen) ?? null;
  const result = preset ? applyPreset(input, preset) : null;

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const created = await presetsApi.create(presetFromInput(name, input));
      setPresets((list) => [...(list ?? []), created]);
      setName("");
    } catch (caught) {
      setError(caught);
    } finally {
      setSaving(false);
    }
  };

  const remove = async (target: Preset) => {
    setError(null);
    try {
      await presetsApi.remove(target.id);
      setPresets(
        (list) => list?.filter((item) => item.id !== target.id) ?? list,
      );
      setChosen(null);
    } catch (caught) {
      setError(caught);
    }
  };

  return (
    <Dialog
      open={open}
      title={PRESET_COPY.title}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            variant="primary"
            disabled={result === null || result.filled.length === 0}
            onClick={() => {
              if (result) {
                onApply(result.next);
              }
              onClose();
            }}
          >
            {PRESET_COPY.apply}
          </Button>
        </>
      }
    >
      <div className="stack">
        <FormError message={formErrorMessage(error, ["name"])} />
        {presets?.length === 0 && <p className="muted">{PRESET_COPY.none}</p>}
        {presets && presets.length > 0 && (
          <fieldset className={styles.presetList}>
            <legend className="visually-hidden">{PRESET_COPY.choose}</legend>
            {presets.map((item) => (
              <label key={item.id} className={styles.presetOption}>
                <input
                  type="radio"
                  name="preset"
                  checked={chosen === item.id}
                  onChange={() => setChosen(item.id)}
                />
                {item.name}
              </label>
            ))}
          </fieldset>
        )}
        {preset && result && (
          <div className={styles.presetPreview} aria-live="polite">
            {result.filled.length === 0 && (
              <p className="muted small">{PRESET_COPY.nothingToFill}</p>
            )}
            {result.filled.length > 0 && (
              <>
                <p className="small">{PRESET_COPY.willFill}</p>
                <ul className="small">
                  {result.filled.map((field) => (
                    <li key={field}>
                      {FIELD_LABELS[field]}: {describeValue(preset, field)}
                    </li>
                  ))}
                </ul>
              </>
            )}
            {result.kept.length > 0 && (
              <p className="muted small">
                {PRESET_COPY.keeps(
                  result.kept.map((field) => FIELD_LABELS[field]).join(", "),
                )}
              </p>
            )}
            {preset.missing.length > 0 && (
              <p className="muted small">
                {PRESET_COPY.missing(preset.missing.join(", "))}
              </p>
            )}
            <div>
              <Button compact variant="ghost" onClick={() => remove(preset)}>
                {PRESET_COPY.remove(preset.name)}
              </Button>
            </div>
          </div>
        )}
        {hasPresetDetails(input) && (
          <form className={styles.presetSave} onSubmit={save}>
            <TextField
              label={PRESET_COPY.saveAs}
              value={name}
              onValueChange={setName}
              placeholder={PRESET_COPY.namePlaceholder}
              hint={PRESET_COPY.saveHint}
              error={fieldErrors(error).name}
            />
            <div>
              <Button type="submit" compact busy={saving}>
                {PRESET_COPY.save}
              </Button>
            </div>
          </form>
        )}
      </div>
    </Dialog>
  );
}
