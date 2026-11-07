import type { Revision } from "../../api/types";
import { ITEM_COPY } from "./copy";
import { formatDay } from "./itemFormat";
import { DETAIL_COPY } from "./itemsCopy";
import styles from "./ItemDetail.module.css";

const FIELD_LABELS: Record<string, string> = {
  name: ITEM_COPY.name,
  description: ITEM_COPY.description,
  quantity: ITEM_COPY.quantity,
  brand: ITEM_COPY.brand,
  model: ITEM_COPY.model,
  serialNumber: ITEM_COPY.serialNumber,
  condition: ITEM_COPY.condition,
  warrantyUntil: ITEM_COPY.warrantyUntil,
  measurements: ITEM_COPY.measurements,
  notes: ITEM_COPY.notes,
  seller: ITEM_COPY.seller,
  orderReference: ITEM_COPY.orderReference,
  pricePaid: ITEM_COPY.pricePaid,
  purchaseDate: ITEM_COPY.purchaseDate,
  room: ITEM_COPY.room,
  category: ITEM_COPY.category,
  tags: ITEM_COPY.tags,
  collections: ITEM_COPY.collections,
  attachments: DETAIL_COPY.attachments,
};

const SOURCE_LABELS: Record<Revision["source"], string> = {
  user: DETAIL_COPY.sourceUser,
  import: DETAIL_COPY.sourceImport,
  restore: DETAIL_COPY.sourceRestore,
};

function describeChange(field: string, change: Revision["changes"][string]) {
  if (field === "created") {
    return DETAIL_COPY.historyCreated;
  }
  if (field === "restored") {
    return DETAIL_COPY.historyRestored;
  }
  const label = FIELD_LABELS[field] ?? field;
  if (typeof change === "boolean") {
    return label;
  }
  if (change.to === "") {
    return DETAIL_COPY.historyCleared(label);
  }
  if (change.from === "") {
    return DETAIL_COPY.historySet(label, change.to);
  }
  return DETAIL_COPY.historyChanged(label, change.from, change.to);
}

export function ItemHistory({
  revisions,
  loading,
}: {
  revisions: Revision[];
  loading: boolean;
}) {
  if (loading) {
    return <p className="muted">{DETAIL_COPY.historyLoading}</p>;
  }
  if (revisions.length === 0) {
    return <p className="muted">{DETAIL_COPY.historyEmpty}</p>;
  }
  return (
    <ol className={styles.history}>
      {revisions.map((revision, index) => (
        <li key={`${revision.changedAt}-${index}`}>
          <ul>
            {Object.entries(revision.changes).map(([field, change]) => (
              <li key={field}>{describeChange(field, change)}</li>
            ))}
          </ul>
          <p className="muted small">
            {DETAIL_COPY.historyMeta(
              formatDay(revision.changedAt),
              SOURCE_LABELS[revision.source],
            )}
          </p>
        </li>
      ))}
    </ol>
  );
}
