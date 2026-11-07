import type { Measurements } from "../../api/types";
import { SelectField } from "../../components/form/SelectField";
import { TextArea, TextField } from "../../components/form/Field";
import { ITEM_COPY, LENGTH_UNITS, WEIGHT_UNITS } from "./copy";
import { measurementsOrNull } from "./measurements";
import styles from "./ItemFields.module.css";

const DEFAULT_LENGTH_UNIT = "cm";
const FIT_NOTE_MAXIMUM = 1000;
/** The count appears once the note gets close to the limit. */
const FIT_NOTE_COUNT_FROM = 800;
const DEFAULT_WEIGHT_UNIT = "kg";

const blank: Measurements = {
  width: "",
  height: "",
  depth: "",
  lengthUnit: DEFAULT_LENGTH_UNIT,
  weight: "",
  weightUnit: DEFAULT_WEIGHT_UNIT,
  fitNote: "",
};

interface MeasurementsFieldsProps {
  value: Measurements | null;
  onChange: (value: Measurements | null) => void;
  error?: string;
  fitNoteError?: string;
}

/** Optional size, weight and a fitting note; every value carries its unit. */
export function MeasurementsFields({
  value,
  onChange,
  error,
  fitNoteError,
}: MeasurementsFieldsProps) {
  const current = {
    ...blank,
    ...value,
    lengthUnit: value?.lengthUnit || DEFAULT_LENGTH_UNIT,
    weightUnit: value?.weightUnit || DEFAULT_WEIGHT_UNIT,
  };
  const set = (field: keyof Measurements, next: string) =>
    onChange(measurementsOrNull({ ...current, [field]: next }));
  const noteLength = [...current.fitNote].length;
  const noteTooLong = noteLength > FIT_NOTE_MAXIMUM;
  const noteHint =
    noteLength >= FIT_NOTE_COUNT_FROM && !noteTooLong
      ? ITEM_COPY.fitNoteCount(noteLength, FIT_NOTE_MAXIMUM)
      : undefined;
  const noteError = noteTooLong
    ? ITEM_COPY.fitNoteTooLong(noteLength, FIT_NOTE_MAXIMUM)
    : fitNoteError;
  const size = [
    { field: "width", label: ITEM_COPY.width },
    { field: "height", label: ITEM_COPY.height },
    { field: "depth", label: ITEM_COPY.depth },
  ] as const;
  return (
    <fieldset className={styles.measurements}>
      <legend>{ITEM_COPY.measurements}</legend>
      {error && (
        <p className={styles.measurementsError} role="alert">
          {error}
        </p>
      )}
      <div className={styles.sizeRow}>
        {size.map(({ field, label }) => (
          <TextField
            key={field}
            label={label}
            inputMode="decimal"
            value={current[field]}
            onValueChange={(next) => set(field, next)}
          />
        ))}
        <SelectField
          label={ITEM_COPY.lengthUnit}
          value={current.lengthUnit}
          options={LENGTH_UNITS}
          onValueChange={(next) => set("lengthUnit", next)}
        />
      </div>
      <div className={styles.weightRow}>
        <TextField
          label={ITEM_COPY.weight}
          inputMode="decimal"
          value={current.weight}
          onValueChange={(next) => set("weight", next)}
        />
        <SelectField
          label={ITEM_COPY.weightUnit}
          value={current.weightUnit}
          options={WEIGHT_UNITS}
          onValueChange={(next) => set("weightUnit", next)}
        />
      </div>
      <TextArea
        label={ITEM_COPY.fitNote}
        optional
        value={current.fitNote}
        onValueChange={(next) => set("fitNote", next)}
        placeholder={ITEM_COPY.fitNotePlaceholder}
        hint={noteHint}
        error={noteError}
      />
    </fieldset>
  );
}
