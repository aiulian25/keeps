import { useId } from "react";
import type { Group } from "../../api/types";
import { Button } from "../../components/Button";
import { CONDITION_OPTIONS, ITEM_COPY } from "./copy";
import { ITEMS_COPY } from "./itemsCopy";
import { PARAMETER } from "./itemsSearchParams";
import styles from "./Items.module.css";

type SelectName =
  | typeof PARAMETER.category
  | typeof PARAMETER.tag
  | typeof PARAMETER.collection;
type FlagName =
  | typeof PARAMETER.hasReceipt
  | typeof PARAMETER.priceKnown
  | typeof PARAMETER.inWarranty;
export type FilterName = SelectName | FlagName | typeof PARAMETER.condition;

interface FilterPanelProps {
  id: string;
  value: (name: FilterName) => string;
  onChange: (name: FilterName, value: string) => void;
  onClear: () => void;
  flagValue: string;
  categories: Group[];
  tags: Group[];
  collections: Group[];
}

function GroupSelect({
  label,
  groups,
  value,
  onChange,
}: {
  label: string;
  groups: Group[];
  value: string;
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className={styles.filterField}>
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        className={styles.select}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{ITEMS_COPY.anyOption}</option>
        {groups.map((group) => (
          <option key={group.id} value={String(group.id)}>
            {group.name}
          </option>
        ))}
      </select>
    </div>
  );
}

function FlagCheckbox({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className={styles.checkbox}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
      />
      {label}
    </label>
  );
}

function ConditionSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className={styles.filterField}>
      <label htmlFor={id}>{ITEM_COPY.condition}</label>
      <select
        id={id}
        className={styles.select}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{ITEMS_COPY.anyOption}</option>
        {CONDITION_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

/** Inferred filter panel (IMP-021): every filter is optional and combines with the others. */
export function FilterPanel({
  id,
  value,
  onChange,
  onClear,
  flagValue,
  categories,
  tags,
  collections,
}: FilterPanelProps) {
  const setFlag = (name: FlagName) => (checked: boolean) =>
    onChange(name, checked ? flagValue : "");
  return (
    <div id={id} className={styles.filterPanel}>
      <GroupSelect
        label={ITEMS_COPY.category}
        groups={categories}
        value={value(PARAMETER.category)}
        onChange={(next) => onChange(PARAMETER.category, next)}
      />
      <GroupSelect
        label={ITEMS_COPY.tag}
        groups={tags}
        value={value(PARAMETER.tag)}
        onChange={(next) => onChange(PARAMETER.tag, next)}
      />
      <GroupSelect
        label={ITEMS_COPY.collection}
        groups={collections}
        value={value(PARAMETER.collection)}
        onChange={(next) => onChange(PARAMETER.collection, next)}
      />
      <ConditionSelect
        value={value(PARAMETER.condition)}
        onChange={(next) => onChange(PARAMETER.condition, next)}
      />
      <div className={styles.filterFlags}>
        <FlagCheckbox
          label={ITEMS_COPY.hasReceipt}
          checked={value(PARAMETER.hasReceipt) === flagValue}
          onChange={setFlag(PARAMETER.hasReceipt)}
        />
        <FlagCheckbox
          label={ITEMS_COPY.priceKnown}
          checked={value(PARAMETER.priceKnown) === flagValue}
          onChange={setFlag(PARAMETER.priceKnown)}
        />
        <FlagCheckbox
          label={ITEM_COPY.inWarranty}
          checked={value(PARAMETER.inWarranty) === flagValue}
          onChange={setFlag(PARAMETER.inWarranty)}
        />
      </div>
      <Button variant="ghost" compact onClick={onClear}>
        {ITEMS_COPY.clearFilters}
      </Button>
    </div>
  );
}
