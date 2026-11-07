import type { ReactNode } from "react";
import type { ItemCondition, ItemInput } from "../../api/types";
import { Disclosure } from "../../components/Bits";
import { DateField } from "../../components/form/DateField";
import { TextArea, TextField } from "../../components/form/Field";
import { GroupInput, MultiGroupInput } from "../../components/form/GroupInput";
import { MoneyField } from "../../components/form/MoneyField";
import { MeasurementsFields } from "./MeasurementsFields";
import { COMMON } from "../../constants/copy";
import { CONDITION_OPTIONS, ITEM_COPY } from "./copy";
import { SelectField } from "../../components/form/SelectField";
import styles from "./ItemFields.module.css";

interface ItemFieldsProps {
  value: ItemInput;
  onChange: (value: ItemInput) => void;
  errors: Record<string, string>;
  currency: string;
  /** Content between the name and the price/date pair (e.g. a listing price). */
  afterName?: ReactNode;
  pricePlaceholder?: string;
  /** Upload areas rendered between the main fields and More details. */
  uploads?: ReactNode;
  /** Extra rows inside More details (e.g. read-only listing price). */
  extraDetails?: ReactNode;
  /** Shown first inside More details, e.g. Use a preset. */
  detailsStart?: ReactNode;
  detailsOpen?: boolean;
  autoFocusName?: boolean;
}

/**
 * The item form from the reference "Add manually" screen: only the name is
 * required; organisation and less common fields wait under More details.
 */
export function ItemFields({
  value,
  onChange,
  errors,
  currency,
  afterName,
  pricePlaceholder = ITEM_COPY.pricePlaceholder,
  uploads,
  extraDetails,
  detailsStart,
  detailsOpen = false,
  autoFocusName = false,
}: ItemFieldsProps) {
  const set = <Key extends keyof ItemInput>(key: Key, next: ItemInput[Key]) =>
    onChange({ ...value, [key]: next });
  return (
    <div className={styles.fields}>
      <TextField
        label={ITEM_COPY.name}
        required
        value={value.name}
        onValueChange={(next) => set("name", next)}
        placeholder={ITEM_COPY.namePlaceholder}
        error={errors.name}
        autoFocus={autoFocusName}
        maxLength={200}
      />
      {afterName}
      <div className={styles.pair}>
        <MoneyField
          label={ITEM_COPY.pricePaid}
          optional
          value={value.pricePaid}
          defaultCurrency={currency}
          onValueChange={(next) => set("pricePaid", next)}
          placeholder={pricePlaceholder}
          error={errors.pricePaid}
        />
        <DateField
          label={ITEM_COPY.purchaseDate}
          optional
          value={value.purchaseDate}
          onValueChange={(next) => set("purchaseDate", next)}
          error={errors.purchaseDate}
        />
      </div>
      <div className={styles.pair}>
        <SelectField
          label={ITEM_COPY.condition}
          optional
          value={value.condition}
          options={[
            { value: "", label: ITEM_COPY.conditionNotSet },
            ...CONDITION_OPTIONS,
          ]}
          onValueChange={(next) => set("condition", next as ItemCondition)}
          error={errors.condition}
        />
        <TextField
          label={ITEM_COPY.warrantyUntil}
          optional
          type="date"
          value={value.warrantyUntil}
          onValueChange={(next) => set("warrantyUntil", next)}
          error={errors.warrantyUntil}
        />
      </div>
      {uploads}
      <Disclosure
        summary={COMMON.moreDetails}
        openSummary={COMMON.fewerDetails}
        defaultOpen={
          detailsOpen ||
          Object.keys(errors).some(
            (key) =>
              key !== "name" && key !== "pricePaid" && key !== "purchaseDate",
          )
        }
      >
        {detailsStart}
        <div className={styles.pair}>
          <GroupInput
            label={ITEM_COPY.room}
            kind="rooms"
            value={value.room}
            onValueChange={(next) => set("room", next)}
            placeholder={ITEM_COPY.roomPlaceholder}
          />
          <GroupInput
            label={ITEM_COPY.category}
            kind="categories"
            value={value.category}
            onValueChange={(next) => set("category", next)}
            placeholder={ITEM_COPY.categoryPlaceholder}
          />
        </div>
        <div className={styles.pair}>
          <MultiGroupInput
            label={ITEM_COPY.tags}
            kind="tags"
            values={value.tags}
            onValuesChange={(next) => set("tags", next)}
            placeholder={ITEM_COPY.tagsPlaceholder}
            removeLabel={ITEM_COPY.removeTag}
          />
          <MultiGroupInput
            label={ITEM_COPY.collections}
            kind="collections"
            values={value.collections}
            onValuesChange={(next) => set("collections", next)}
            placeholder={ITEM_COPY.collectionsPlaceholder}
            removeLabel={ITEM_COPY.removeTag}
          />
        </div>
        <div className={styles.pair}>
          <TextField
            label={ITEM_COPY.seller}
            optional
            value={value.seller}
            onValueChange={(next) => set("seller", next)}
            placeholder={ITEM_COPY.sellerPlaceholder}
            error={errors.seller}
          />
          <TextField
            label={ITEM_COPY.orderReference}
            optional
            value={value.orderReference}
            onValueChange={(next) => set("orderReference", next)}
            error={errors.orderReference}
          />
        </div>
        <MeasurementsFields
          value={value.measurements}
          onChange={(next) => set("measurements", next)}
          error={errors.measurements}
        />
        {extraDetails}
        <div className={styles.triple}>
          <TextField
            label={ITEM_COPY.brand}
            optional
            value={value.brand}
            onValueChange={(next) => set("brand", next)}
            error={errors.brand}
          />
          <TextField
            label={ITEM_COPY.model}
            optional
            value={value.model}
            onValueChange={(next) => set("model", next)}
            error={errors.model}
          />
          <TextField
            label={ITEM_COPY.serialNumber}
            optional
            value={value.serialNumber}
            onValueChange={(next) => set("serialNumber", next)}
            error={errors.serialNumber}
          />
        </div>
        <div className={styles.quantity}>
          <TextField
            label={ITEM_COPY.quantity}
            type="number"
            min={1}
            inputMode="numeric"
            value={String(value.quantity)}
            onValueChange={(next) =>
              set("quantity", Number.parseInt(next, 10) || 1)
            }
            error={errors.quantity}
          />
        </div>
        <TextArea
          label={ITEM_COPY.description}
          optional
          value={value.description}
          onValueChange={(next) => set("description", next)}
          error={errors.description}
          rows={3}
        />
        <TextArea
          label={ITEM_COPY.notes}
          optional
          value={value.notes}
          onValueChange={(next) => set("notes", next)}
          error={errors.notes}
          rows={3}
        />
      </Disclosure>
    </div>
  );
}
