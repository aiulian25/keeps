import { Home, ImageOff } from "lucide-react";
import { Link, useNavigate } from "react-router";
import { attachmentUrl } from "../../api/client";
import type { ItemSummary } from "../../api/types";
import { OverflowMenu } from "../../components/Bits";
import { ROUTES } from "../../constants/routes";
import { formatMoney, formatPurchaseDate } from "./itemFormat";
import { ITEMS_COPY } from "./itemsCopy";
import type { ItemDetailState } from "./navigationState";
import styles from "./Items.module.css";

export type ItemView = "grid" | "list";

const SKELETON_COUNT = 8;
const editState: ItemDetailState = { edit: true };

interface ItemCollectionProps {
  items: ItemSummary[];
  view: ItemView;
  onTrash: (item: ItemSummary) => void;
}

function Thumbnail({
  item,
  className,
}: {
  item: ItemSummary;
  className: string;
}) {
  if (item.primaryPhotoId === 0) {
    return (
      <div className={`${className} ${styles.noPhoto}`}>
        <ImageOff aria-hidden="true" />
      </div>
    );
  }
  // The name is right next to the photo, so the image itself is decorative here.
  return (
    <img
      className={className}
      src={attachmentUrl(item.primaryPhotoId, "thumbnail")}
      alt=""
      loading="lazy"
    />
  );
}

function ItemActions({
  item,
  onTrash,
}: {
  item: ItemSummary;
  onTrash: (item: ItemSummary) => void;
}) {
  const navigate = useNavigate();
  return (
    <div className={styles.actions}>
      <OverflowMenu
        label={ITEMS_COPY.itemActions(item.name)}
        actions={[
          {
            label: ITEMS_COPY.open,
            onSelect: () => navigate(ROUTES.item(item.id)),
          },
          {
            label: ITEMS_COPY.edit,
            onSelect: () =>
              navigate(ROUTES.item(item.id), { state: editState }),
          },
          {
            label: ITEMS_COPY.moveToTrash,
            onSelect: () => onTrash(item),
            tone: "danger",
          },
        ]}
      />
    </div>
  );
}

function ItemCard({
  item,
  onTrash,
}: {
  item: ItemSummary;
  onTrash: (item: ItemSummary) => void;
}) {
  return (
    <li className={styles.card}>
      <Thumbnail item={item} className={styles.cardPhoto} />
      <div className={styles.cardBody}>
        <h3 className={styles.cardName}>
          <Link to={ROUTES.item(item.id)} className={styles.coverLink}>
            {item.name}
          </Link>
        </h3>
        {item.pricePaid && (
          <p className={styles.price}>{formatMoney(item.pricePaid)}</p>
        )}
        <div className={styles.cardFooter}>
          {item.room && (
            <span className={styles.roomTag}>
              <Home aria-hidden="true" />
              {item.room.name}
            </span>
          )}
          <ItemActions item={item} onTrash={onTrash} />
        </div>
      </div>
    </li>
  );
}

function ItemRow({
  item,
  onTrash,
}: {
  item: ItemSummary;
  onTrash: (item: ItemSummary) => void;
}) {
  return (
    <li className={styles.row}>
      <Thumbnail item={item} className={styles.rowPhoto} />
      <Link
        to={ROUTES.item(item.id)}
        className={`${styles.coverLink} ${styles.rowName}`}
      >
        {item.name}
      </Link>
      <span className={styles.rowMeta}>{item.room?.name}</span>
      <span className={styles.rowMeta}>
        {item.purchaseDate && formatPurchaseDate(item.purchaseDate)}
      </span>
      <span className={styles.rowPrice}>
        {item.pricePaid && formatMoney(item.pricePaid)}
      </span>
      <ItemActions item={item} onTrash={onTrash} />
    </li>
  );
}

/** Photo cards (panel 03) or compact rows (UX-04); each item is one link. */
export function ItemCollection({ items, view, onTrash }: ItemCollectionProps) {
  if (view === "list") {
    return (
      <ul className={styles.list}>
        {items.map((item) => (
          <ItemRow key={item.id} item={item} onTrash={onTrash} />
        ))}
      </ul>
    );
  }
  return (
    <ul className={styles.grid}>
      {items.map((item) => (
        <ItemCard key={item.id} item={item} onTrash={onTrash} />
      ))}
    </ul>
  );
}

export function ItemSkeleton({
  view,
  count = SKELETON_COUNT,
}: {
  view: ItemView;
  count?: number;
}) {
  const placeholders = Array.from({ length: count }, (_, index) => index);
  const listClass = view === "list" ? styles.list : styles.grid;
  const itemClass = view === "list" ? styles.skeletonRow : styles.skeletonCard;
  return (
    <ul className={listClass} aria-hidden="true">
      {placeholders.map((index) => (
        <li key={index} className={itemClass} />
      ))}
    </ul>
  );
}
