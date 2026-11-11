import { useState, type FormEvent, type ReactNode } from "react";
import {
  DoorOpen,
  Ruler,
  Weight,
  Gauge,
  ShieldCheck,
  CalendarDays,
  FileText,
  Home,
  Layers,
  Pencil,
  Plus,
  StickyNote,
  Tag,
} from "lucide-react";
import { Link } from "react-router";
import { ApiError } from "../../api/client";
import type { Item, Money } from "../../api/types";
import { Chip } from "../../components/Bits";
import { Button, IconButton } from "../../components/Button";
import { MoneyField } from "../../components/form/MoneyField";
import { COMMON, ERRORS } from "../../constants/copy";
import { SINGLE_QUANTITY } from "../../constants/limits";
import { ROUTES } from "../../constants/routes";
import { formatSize, formatWeight } from "./measurements";
import { conditionLabel, ITEM_COPY } from "./copy";
import {
  formatDay,
  formatDayMonth,
  formatMoney,
  formatPurchaseDate,
} from "./itemFormat";

const DATE_LENGTH = 10;
import { DETAIL_COPY } from "./itemsCopy";
import { LongText } from "./LongText";
import styles from "./ItemDetail.module.css";

interface FactProps {
  label: string;
  icon?: ReactNode;
  children: ReactNode;
  action?: ReactNode;
}

function Fact({ label, icon, children, action }: FactProps) {
  return (
    <div className={styles.fact}>
      <dt>{label}</dt>
      <dd>
        <span className={styles.factValue}>
          {icon}
          {children}
        </span>
        {action}
      </dd>
    </div>
  );
}

/** Unknown values offer a way to fill them in rather than a guess. */
/** Works out the status from the date, so it never goes out of date. */
function WarrantyStatus({ until }: { until: string }) {
  const today = new Date().toISOString().slice(0, DATE_LENGTH);
  const covered = until >= today;
  const date = formatDay(until);
  return (
    <span className={covered ? styles.warrantyActive : undefined}>
      {covered
        ? ITEM_COPY.inWarrantyUntil(date)
        : ITEM_COPY.warrantyEnded(date)}
    </span>
  );
}

function Unknown({ onAdd }: { onAdd: () => void }) {
  return (
    <button type="button" className={styles.addLink} onClick={onAdd}>
      {DETAIL_COPY.add}
    </button>
  );
}

interface PriceEditorProps {
  value: Money | null;
  currency: string;
  onSave: (price: Money | null) => Promise<void>;
  onDone: () => void;
}

function PriceEditor({ value, currency, onSave, onDone }: PriceEditorProps) {
  const [draft, setDraft] = useState(value);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onSave(draft);
      onDone();
    } catch (caught) {
      setError(
        caught instanceof ApiError
          ? (caught.fields.pricePaid ?? caught.message)
          : ERRORS.generic,
      );
    } finally {
      setBusy(false);
    }
  };
  return (
    <form className={styles.priceEditor} onSubmit={submit}>
      <MoneyField
        label={ITEM_COPY.pricePaid}
        value={draft}
        defaultCurrency={currency}
        onValueChange={setDraft}
        placeholder={ITEM_COPY.pricePlaceholder}
        error={error || undefined}
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
        <Button compact onClick={onDone}>
          {COMMON.cancel}
        </Button>
      </div>
    </form>
  );
}

function names(references: { name: string }[]) {
  return references
    .map((reference) => reference.name)
    .join(DETAIL_COPY.listSeparator);
}

interface ItemFactsProps {
  item: Item;
  currency: string;
  onEdit: () => void;
  onSavePrice: (price: Money | null) => Promise<void>;
}

/** Panel 08 label/value list, then the less common details when present. */
export function ItemFacts({
  item,
  currency,
  onEdit,
  onSavePrice,
}: ItemFactsProps) {
  const [editingPrice, setEditingPrice] = useState(false);
  const iconProps = {
    "aria-hidden": true,
    className: styles.factIcon,
  } as const;
  const editPrice = () => setEditingPrice(true);
  const documentCount = item.documents.length;

  return (
    <div className="stack">
      {editingPrice && (
        <PriceEditor
          value={item.pricePaid}
          currency={currency}
          onSave={onSavePrice}
          onDone={() => setEditingPrice(false)}
        />
      )}
      <dl className={styles.facts}>
        {!editingPrice && (
          <Fact
            label={ITEM_COPY.pricePaid}
            action={
              <IconButton label={DETAIL_COPY.editPrice} onClick={editPrice}>
                <Pencil aria-hidden="true" />
              </IconButton>
            }
          >
            {item.pricePaid ? (
              <strong>{formatMoney(item.pricePaid)}</strong>
            ) : (
              <Unknown onAdd={editPrice} />
            )}
          </Fact>
        )}
        <Fact
          label={ITEM_COPY.purchaseDate}
          icon={<CalendarDays {...iconProps} />}
        >
          {item.purchaseDate ? (
            formatPurchaseDate(item.purchaseDate)
          ) : (
            <Unknown onAdd={onEdit} />
          )}
        </Fact>
        <Fact label={ITEM_COPY.condition} icon={<Gauge {...iconProps} />}>
          {item.condition ? (
            conditionLabel(item.condition)
          ) : (
            <Unknown onAdd={onEdit} />
          )}
        </Fact>
        {item.warrantyUntil && (
          <Fact
            label={ITEM_COPY.warranty}
            icon={<ShieldCheck {...iconProps} />}
          >
            <WarrantyStatus until={item.warrantyUntil} />
          </Fact>
        )}
        {item.measurements && formatSize(item.measurements) && (
          <Fact label={ITEM_COPY.size} icon={<Ruler {...iconProps} />}>
            {formatSize(item.measurements)}
          </Fact>
        )}
        {item.measurements && formatWeight(item.measurements) && (
          <Fact label={ITEM_COPY.weight} icon={<Weight {...iconProps} />}>
            {formatWeight(item.measurements)}
          </Fact>
        )}
        {item.measurements?.fitNote && (
          <Fact label={ITEM_COPY.fitNote} icon={<DoorOpen {...iconProps} />}>
            {item.measurements.fitNote}
          </Fact>
        )}
        <Fact label={ITEM_COPY.room} icon={<Home {...iconProps} />}>
          {item.room ? item.room.name : <Unknown onAdd={onEdit} />}
        </Fact>
        <Fact label={ITEM_COPY.category} icon={<Tag {...iconProps} />}>
          {item.category ? item.category.name : <Unknown onAdd={onEdit} />}
        </Fact>
        <Fact
          label={ITEM_COPY.tags}
          action={
            <IconButton label={DETAIL_COPY.addTag} onClick={onEdit}>
              <Plus aria-hidden="true" />
            </IconButton>
          }
        >
          <span className={styles.chips}>
            {item.tags.map((tag) => (
              <Chip key={tag.id}>{tag.name}</Chip>
            ))}
            {item.tags.length === 0 && COMMON.unknown}
          </span>
        </Fact>
        <Fact label={ITEM_COPY.collections} icon={<Layers {...iconProps} />}>
          {item.collections.length > 0 ? (
            names(item.collections)
          ) : (
            <Unknown onAdd={onEdit} />
          )}
        </Fact>
        <Fact label={ITEM_COPY.notes} icon={<StickyNote {...iconProps} />}>
          {item.notes ? (
            <span className={styles.preWrap}>{item.notes}</span>
          ) : (
            <Unknown onAdd={onEdit} />
          )}
        </Fact>
        {item.seller && <Fact label={ITEM_COPY.seller}>{item.seller}</Fact>}
        {item.orderReference && (
          <Fact label={ITEM_COPY.orderReference}>{item.orderReference}</Fact>
        )}
        {item.brand && <Fact label={ITEM_COPY.brand}>{item.brand}</Fact>}
        {item.model && <Fact label={ITEM_COPY.model}>{item.model}</Fact>}
        {item.serialNumber && (
          <Fact label={ITEM_COPY.serialNumber}>{item.serialNumber}</Fact>
        )}
        {item.quantity > SINGLE_QUANTITY && (
          <Fact label={ITEM_COPY.quantity}>{item.quantity}</Fact>
        )}
        {item.description && (
          <Fact label={ITEM_COPY.description}>
            <LongText text={item.description} />
          </Fact>
        )}
        <Fact label={DETAIL_COPY.documents} icon={<FileText {...iconProps} />}>
          <Link to={ROUTES.itemDocuments(item.id)}>
            {DETAIL_COPY.documents}
          </Link>
          <span className="muted">
            {DETAIL_COPY.partSeparator}
            {DETAIL_COPY.documentCount(documentCount)}
          </span>
        </Fact>
      </dl>
      {item.listing && <ListingLine listing={item.listing} />}
      {item.sharedWith.length > 0 && (
        <div className={styles.shared}>
          <p>
            {DETAIL_COPY.boughtWith}{" "}
            {item.sharedWith.map((other, index) => (
              <span key={other.id}>
                {index > 0 && DETAIL_COPY.listSeparator}
                <Link to={ROUTES.item(other.id)}>{other.name}</Link>
              </span>
            ))}
          </p>
          <p className="muted small">{DETAIL_COPY.invoiceShared}</p>
        </div>
      )}
    </div>
  );
}

/** Listing data stays visibly separate from what was actually paid (DEV-08, DEV-10). */
function ListingLine({ listing }: { listing: NonNullable<Item["listing"]> }) {
  const parts = [
    listing.price ? DETAIL_COPY.listingPrice(formatMoney(listing.price)) : "",
    DETAIL_COPY.listingFrom(listing.sourceHost),
    DETAIL_COPY.fetchedOn(formatDayMonth(listing.retrievedAt)),
  ].filter(Boolean);
  return (
    <p className={`muted small ${styles.listing}`}>
      {parts.join(DETAIL_COPY.partSeparator)}
    </p>
  );
}
