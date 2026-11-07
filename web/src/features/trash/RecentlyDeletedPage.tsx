import { useState } from "react";
import { Package } from "lucide-react";
import { useNavigate } from "react-router";
import { attachmentUrl } from "../../api/client";
import type { ItemSummary } from "../../api/types";
import { LoadingLine } from "../../components/Bits";
import { Button } from "../../components/Button";
import { FormError } from "../../components/form/Field";
import { PageHeader } from "../../components/shell/PageHeader";
import { useToast } from "../../components/Toast";
import { ROUTES } from "../../constants/routes";
import { ConfirmDialog } from "../account/ConfirmDialog";
import { errorMessage } from "../account/errors";
import { formatShortDate } from "../account/format";
import { TRASH_COPY } from "./copy";
import { useDeletedItems } from "./useDeletedItems";
import styles from "./Trash.module.css";

interface DeletedRowProps {
  item: ItemSummary;
  restoring: boolean;
  onRestore: () => void;
  onPurge: () => void;
}

function DeletedRow({ item, restoring, onRestore, onPurge }: DeletedRowProps) {
  return (
    <li className={styles.row}>
      {item.primaryPhotoId ? (
        <img
          className={styles.thumbnail}
          src={attachmentUrl(item.primaryPhotoId, "thumbnail")}
          alt=""
          loading="lazy"
        />
      ) : (
        <span className={styles.thumbnail} aria-hidden="true">
          <Package />
        </span>
      )}
      <div className={styles.text}>
        <span className={styles.name}>{item.name}</span>
        {item.deletedAt && (
          <span className="muted small">
            {TRASH_COPY.deletedOn(formatShortDate(item.deletedAt))}
          </span>
        )}
      </div>
      <div className={styles.actions}>
        <Button
          compact
          busy={restoring}
          onClick={onRestore}
          aria-label={TRASH_COPY.restoreLabel(item.name)}
        >
          {TRASH_COPY.restore}
        </Button>
        <Button
          compact
          variant="danger"
          onClick={onPurge}
          aria-label={TRASH_COPY.deletePermanentlyLabel(item.name)}
        >
          {TRASH_COPY.deletePermanently}
        </Button>
      </div>
    </li>
  );
}

/** UX-17: restore deleted items, or delete them for good after confirming. */
export function RecentlyDeletedPage() {
  const { items, loadError, hasMore, loadMore, restore, purge } =
    useDeletedItems();
  const { notify } = useToast();
  const navigate = useNavigate();
  const [restoringId, setRestoringId] = useState<number | null>(null);
  const [purging, setPurging] = useState<ItemSummary | null>(null);

  const restoreItem = async (item: ItemSummary) => {
    setRestoringId(item.id);
    try {
      await restore(item.id);
      notify({
        title: TRASH_COPY.restored(item.name),
        action: {
          label: TRASH_COPY.view,
          onAction: () => navigate(ROUTES.item(item.id)),
        },
      });
    } catch (caught) {
      notify({ title: errorMessage(caught), tone: "error" });
    } finally {
      setRestoringId(null);
    }
  };

  return (
    <div className={styles.page}>
      <PageHeader
        title={TRASH_COPY.title}
        backTo={ROUTES.items}
        backLabel={TRASH_COPY.backLabel}
      />
      <FormError message={loadError} />
      {items === null && !loadError && <LoadingLine />}
      {items?.length === 0 && (
        <p className={styles.empty}>{TRASH_COPY.empty}</p>
      )}
      {items && items.length > 0 && (
        <ul className={styles.list}>
          {items.map((item) => (
            <DeletedRow
              key={item.id}
              item={item}
              restoring={restoringId === item.id}
              onRestore={() => restoreItem(item)}
              onPurge={() => setPurging(item)}
            />
          ))}
        </ul>
      )}
      {hasMore && (
        <div>
          <Button onClick={loadMore}>{TRASH_COPY.showMore}</Button>
        </div>
      )}
      <ConfirmDialog
        open={purging !== null}
        title={TRASH_COPY.confirmTitle(purging?.name ?? "")}
        text={TRASH_COPY.confirmText}
        confirmLabel={TRASH_COPY.deletePermanently}
        danger
        onConfirm={async () => {
          await purge(purging?.id ?? 0);
          notify({ title: TRASH_COPY.purged(purging?.name ?? "") });
        }}
        onClose={() => setPurging(null)}
      />
    </div>
  );
}
