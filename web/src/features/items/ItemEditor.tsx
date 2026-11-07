import { useEffect, useRef, type FormEvent } from "react";
import type { ItemInput } from "../../api/types";
import { Button } from "../../components/Button";
import { FormError } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { conditionLabel, ITEM_COPY } from "./copy";
import { formatMeasurements } from "./measurements";
import { ItemFields } from "./ItemFields";
import { formatDay, formatMoney, formatPurchaseDate } from "./itemFormat";
import { DETAIL_COPY } from "./itemsCopy";
import styles from "./ItemDetail.module.css";

type ComparableField = Exclude<keyof ItemInput, "purchaseId" | "attachmentIds">;

// A Record, so every editable field must be listed: one left out here
// would silently lose the person's version in a conflict.
const COMPARABLE_LABELS: Record<ComparableField, string> = {
  name: ITEM_COPY.name,
  pricePaid: ITEM_COPY.pricePaid,
  purchaseDate: ITEM_COPY.purchaseDate,
  condition: ITEM_COPY.condition,
  warrantyUntil: ITEM_COPY.warrantyUntil,
  measurements: ITEM_COPY.measurements,
  room: ITEM_COPY.room,
  category: ITEM_COPY.category,
  tags: ITEM_COPY.tags,
  collections: ITEM_COPY.collections,
  seller: ITEM_COPY.seller,
  orderReference: ITEM_COPY.orderReference,
  brand: ITEM_COPY.brand,
  model: ITEM_COPY.model,
  serialNumber: ITEM_COPY.serialNumber,
  quantity: ITEM_COPY.quantity,
  description: ITEM_COPY.description,
  notes: ITEM_COPY.notes,
};

const COMPARABLE_FIELDS = (
  Object.entries(COMPARABLE_LABELS) as [ComparableField, string][]
).map(([field, label]) => ({ field, label }));

function sameValue(first: unknown, second: unknown) {
  return JSON.stringify(first) === JSON.stringify(second);
}

function displayValue(input: ItemInput, field: ComparableField) {
  const value = input[field];
  if (field === "pricePaid" && input.pricePaid) {
    return formatMoney(input.pricePaid);
  }
  if (field === "purchaseDate" && input.purchaseDate) {
    return formatPurchaseDate(input.purchaseDate);
  }
  if (field === "condition" && input.condition) {
    return conditionLabel(input.condition);
  }
  if (field === "warrantyUntil" && input.warrantyUntil) {
    return formatDay(input.warrantyUntil);
  }
  if (field === "measurements") {
    return formatMeasurements(input.measurements) || COMMON.unknown;
  }
  if (Array.isArray(value)) {
    return value.join(DETAIL_COPY.listSeparator) || COMMON.unknown;
  }
  if (value === null || value === "") {
    return COMMON.unknown;
  }
  return String(value);
}

/** Fields where the person's unsaved version differs from the latest saved one and hasn't been taken over yet. */
export function conflictingFields(
  mine: ItemInput,
  latest: ItemInput,
  draft: ItemInput,
) {
  return COMPARABLE_FIELDS.filter(
    ({ field }) =>
      !sameValue(mine[field], latest[field]) &&
      !sameValue(draft[field], mine[field]),
  );
}

export type EditConflict =
  | { stage: "detected" }
  | { stage: "compare"; mine: ItemInput; latest: ItemInput };

interface ItemEditorProps {
  draft: ItemInput;
  onChange: (draft: ItemInput) => void;
  errors: Record<string, string>;
  formError: string;
  currency: string;
  busy: boolean;
  conflict: EditConflict | null;
  reloading: boolean;
  onSave: () => void;
  onCancel: () => void;
  onReload: () => void;
}

function ConflictNotice({
  conflict,
  draft,
  reloading,
  onReload,
  onUseMine,
}: {
  conflict: EditConflict;
  draft: ItemInput;
  reloading: boolean;
  onReload: () => void;
  onUseMine: (field: ComparableField, mine: ItemInput) => void;
}) {
  if (conflict.stage === "detected") {
    return (
      <div className={styles.notice} role="alert">
        <p>{DETAIL_COPY.conflictTitle}</p>
        <Button
          compact
          onClick={onReload}
          busy={reloading}
          busyLabel={COMMON.loading}
        >
          {DETAIL_COPY.conflictReload}
        </Button>
      </div>
    );
  }
  const differences = conflictingFields(conflict.mine, conflict.latest, draft);
  if (differences.length === 0) {
    return null;
  }
  return (
    <div className={styles.notice} role="status">
      <p>{DETAIL_COPY.conflictExplain}</p>
      <table className={styles.compare}>
        <thead>
          <tr>
            <td />
            <th scope="col">{DETAIL_COPY.yourVersion}</th>
            <th scope="col">{DETAIL_COPY.latestVersion}</th>
            <td />
          </tr>
        </thead>
        <tbody>
          {differences.map(({ field, label }) => (
            <tr key={field}>
              <th scope="row">{label}</th>
              <td>{displayValue(conflict.mine, field)}</td>
              <td>{displayValue(conflict.latest, field)}</td>
              <td>
                <Button
                  compact
                  variant="ghost"
                  onClick={() => onUseMine(field, conflict.mine)}
                >
                  {DETAIL_COPY.useMine}
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** UX-11 edit mode: the same fields as Add manually, all details open. */
export function ItemEditor({
  draft,
  onChange,
  errors,
  formError,
  currency,
  busy,
  conflict,
  reloading,
  onSave,
  onCancel,
  onReload,
}: ItemEditorProps) {
  const submit = (event: FormEvent) => {
    event.preventDefault();
    onSave();
  };
  const form = useRef<HTMLFormElement>(null);
  // Ctrl/Cmd+Enter saves and Escape cancels from anywhere inside the form.
  useEffect(() => {
    const element = form.current;
    const onKeyDown = (event: KeyboardEvent) => {
      const saveShortcut =
        event.key === "Enter" && (event.ctrlKey || event.metaKey);
      if (saveShortcut) {
        event.preventDefault();
        onSave();
        return;
      }
      if (event.key === "Escape" && !event.defaultPrevented) {
        event.preventDefault();
        onCancel();
      }
    };
    element?.addEventListener("keydown", onKeyDown);
    return () => element?.removeEventListener("keydown", onKeyDown);
  }, [onSave, onCancel]);
  const takeMine = (field: ComparableField, mine: ItemInput) =>
    onChange({ ...draft, [field]: mine[field] });

  return (
    <form
      ref={form}
      className={styles.editor}
      onSubmit={submit}
      aria-label={DETAIL_COPY.editForm}
      noValidate
    >
      {conflict && (
        <ConflictNotice
          conflict={conflict}
          draft={draft}
          reloading={reloading}
          onReload={onReload}
          onUseMine={takeMine}
        />
      )}
      <FormError message={formError} />
      <ItemFields
        value={draft}
        onChange={onChange}
        errors={errors}
        currency={currency}
        detailsOpen
      />
      <div className={styles.editorActions}>
        <Button
          type="submit"
          variant="primary"
          busy={busy}
          busyLabel={COMMON.saving}
        >
          {ITEM_COPY.saveChanges}
        </Button>
        <Button onClick={onCancel}>{COMMON.cancel}</Button>
      </div>
    </form>
  );
}
