import { useCallback, useEffect, useRef, useState } from "react";
import { Camera, Pencil } from "lucide-react";
import {
  Link,
  useBlocker,
  useLocation,
  useNavigate,
  useParams,
} from "react-router";
import { ApiError } from "../../api/client";
import { itemsApi } from "../../api/endpoints";
import type { Attachment, Item, ItemInput, Money } from "../../api/types";
import {
  Disclosure,
  LoadingLine,
  OverflowMenu,
  SavedBadge,
} from "../../components/Bits";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { useToast } from "../../components/Toast";
import { PageHeader } from "../../components/shell/PageHeader";
import { COMMON, ERRORS } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import type { ManualItemState } from "../add/manualState";
import { UPLOAD_COPY } from "../attachments/copy";
import { DropZone } from "../attachments/DropZone";
import { useUploads } from "../attachments/useUploads";
import { useSession } from "../../session/SessionContext";
import { ItemEditor, type EditConflict } from "./ItemEditor";
import { ItemFacts } from "./ItemFacts";
import { CompatibleParts } from "./CompatibleParts";
import { ItemHistory } from "./ItemHistory";
import { inputFromItem, sameInput } from "./itemInput";
import { DETAIL_COPY } from "./itemsCopy";
import { itemDetailState } from "./navigationState";
import { PhotoGallery } from "./PhotoGallery";
import { useItem, useItemHistory } from "./useItem";
import { useTrashItem } from "./useTrashItem";
import styles from "./ItemDetail.module.css";

const CONFLICT_CODE = "edit_conflict";
const STATUS_CONFLICT = 409;

type Mode = "view" | "edit";

/** Panel 08 plus UX-10/UX-11: view, inline price edit, full edit with conflict handling. */
export function ItemDetailPage() {
  const { itemId: itemIdText } = useParams();
  const itemId = Number(itemIdText);
  const { item, loading, problem, replace, reload } = useItem(itemId);

  if (problem === "not-found" || !Number.isInteger(itemId)) {
    return (
      <div className="stack">
        <PageHeader
          title={DETAIL_COPY.notFoundTitle}
          backTo={ROUTES.items}
          backLabel={DETAIL_COPY.backToItems}
        />
        <p className="muted">{DETAIL_COPY.notFoundText}</p>
        <p>
          <Link to={ROUTES.items}>{DETAIL_COPY.backToItems}</Link>
        </p>
      </div>
    );
  }
  if (loading) {
    return <LoadingLine />;
  }
  if (!item) {
    return (
      <div className="stack" role="alert">
        <PageHeader
          title={DETAIL_COPY.loadFailed}
          backTo={ROUTES.items}
          backLabel={DETAIL_COPY.backToItems}
        />
        <div>
          <Button onClick={() => reload().catch(() => undefined)}>
            {COMMON.retry}
          </Button>
        </div>
      </div>
    );
  }
  return (
    <ItemDetail
      key={item.id}
      item={item}
      onItemChange={replace}
      reload={reload}
    />
  );
}

interface ItemDetailProps {
  item: Item;
  onItemChange: (item: Item) => void;
  reload: () => Promise<Item>;
}

function ItemDetail({ item, onItemChange, reload }: ItemDetailProps) {
  const navigate = useNavigate();
  const incoming = itemDetailState(useLocation().state);
  const { account } = useSession();
  const { notify } = useToast();
  const trashItem = useTrashItem();
  const currency = item.pricePaid?.currency ?? account?.currency ?? "";
  const history = useItemHistory(item.id, item.revision);

  const [mode, setMode] = useState<Mode>(incoming.edit ? "edit" : "view");
  const [savedToken, setSavedToken] = useState<number | null>(
    incoming.saved ? 1 : null,
  );
  const [baseline, setBaseline] = useState<ItemInput>(() =>
    inputFromItem(item),
  );
  const [draft, setDraft] = useState<ItemInput>(baseline);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState("");
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState<EditConflict | null>(null);
  const [reloading, setReloading] = useState(false);
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [duplicatesDismissed, setDuplicatesDismissed] = useState(false);
  const [historyOpenCount, setHistoryOpenCount] = useState(0);
  const heading = useRef<HTMLSpanElement>(null);
  const historySection = useRef<HTMLDivElement>(null);

  const dirty =
    mode === "edit" && (!sameInput(draft, baseline) || conflict !== null);
  const blocker = useBlocker(dirty);

  useEffect(() => {
    if (!dirty) {
      return;
    }
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const markSaved = (saved: Item) => {
    onItemChange(saved);
    setSavedToken((token) => (token ?? 0) + 1);
  };

  const startEdit = () => {
    const input = inputFromItem(item);
    setBaseline(input);
    setDraft(input);
    setMode("edit");
  };

  const exitEdit = () => {
    setMode("view");
    setConflict(null);
    setErrors({});
    setFormError("");
  };

  const cancelEdit = () => {
    if (dirty) {
      setConfirmCancel(true);
      return;
    }
    exitEdit();
  };

  const showSaveError = (error: unknown) => {
    if (error instanceof ApiError && error.code === CONFLICT_CODE) {
      setConflict({ stage: "detected" });
      return;
    }
    if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
      setErrors(error.fields);
      setFormError(ERRORS.checkFields);
      return;
    }
    setFormError(error instanceof ApiError ? error.message : ERRORS.generic);
  };

  const save = async () => {
    if (busy) {
      return;
    }
    setBusy(true);
    setErrors({});
    setFormError("");
    try {
      markSaved(await itemsApi.update(item.id, item.revision, draft));
      exitEdit();
      window.requestAnimationFrame(() => heading.current?.focus());
    } catch (error) {
      showSaveError(error);
    } finally {
      setBusy(false);
    }
  };

  const reloadAfterConflict = async () => {
    setReloading(true);
    try {
      const latest = inputFromItem(await reload());
      setConflict({ stage: "compare", mine: draft, latest });
      setBaseline(latest);
      setDraft(latest);
    } catch {
      setFormError(ERRORS.generic);
    } finally {
      setReloading(false);
    }
  };

  const savePrice = async (pricePaid: Money | null) => {
    try {
      markSaved(
        await itemsApi.update(item.id, item.revision, {
          ...inputFromItem(item),
          pricePaid,
        }),
      );
    } catch (error) {
      const conflicted =
        error instanceof ApiError && error.code === CONFLICT_CODE;
      if (!conflicted) {
        throw error;
      }
      await reload();
      throw new ApiError(
        STATUS_CONFLICT,
        CONFLICT_CODE,
        DETAIL_COPY.priceConflict,
      );
    }
  };

  const changePhotos = (change: Promise<void>, success?: () => void) =>
    change
      .then(reload)
      .then(() => success?.())
      .catch(() => notify({ title: DETAIL_COPY.photoFailed, tone: "error" }));

  const setMainPhoto = (photo: Attachment) =>
    changePhotos(itemsApi.setPrimaryPhoto(item.id, photo.id));
  const removePhoto = (photo: Attachment) =>
    changePhotos(itemsApi.unlinkAttachment(item.id, photo.id), () =>
      notify({
        title: DETAIL_COPY.photoRemoved,
        action: {
          label: COMMON.undo,
          onAction: () =>
            changePhotos(itemsApi.linkAttachments(item.id, [photo.id])),
        },
      }),
    );

  const itemIdentifier = item.id;
  // Rejects only when the photo was not added, so the upload row can offer
  // Retry; a failed refresh afterwards is reported on its own.
  const linkUploadedPhoto = useCallback(
    (attachment: Attachment) =>
      itemsApi
        .linkAttachments(itemIdentifier, [attachment.id])
        .then(() =>
          reload().catch(() =>
            notify({ title: UPLOAD_COPY.refreshFailed, tone: "error" }),
          ),
        ),
    [itemIdentifier, reload, notify],
  );
  const uploads = useUploads(linkUploadedPhoto);

  const trash = () =>
    trashItem(item.id, {
      onTrashed: () => navigate(ROUTES.items),
      onRestored: () => navigate(ROUTES.item(item.id)),
    });

  const showHistory = () => {
    setHistoryOpenCount((count) => count + 1);
    window.requestAnimationFrame(() =>
      historySection.current?.scrollIntoView({ block: "start" }),
    );
  };

  const addFromPurchase = () => {
    const state: ManualItemState = {
      purchaseId: item.purchaseId,
      purchaseItemName: item.name,
      seller: item.seller,
      orderReference: item.orderReference,
      purchaseDate: item.purchaseDate,
    };
    navigate(ROUTES.addManual, { state });
  };

  const discardOpen = blocker.state === "blocked" || confirmCancel;
  const keepEditing = () => {
    blocker.reset?.();
    setConfirmCancel(false);
  };
  const discard = () => {
    setConfirmCancel(false);
    if (blocker.state === "blocked") {
      blocker.proceed?.();
      return;
    }
    exitEdit();
  };

  const duplicates = incoming.possibleDuplicates ?? [];
  const showDuplicates = duplicates.length > 0 && !duplicatesDismissed;

  return (
    <div className="stack">
      <PageHeader
        backTo={ROUTES.items}
        backLabel={DETAIL_COPY.backToItems}
        title={
          <span ref={heading} tabIndex={-1} className={styles.headingTarget}>
            {item.name}
          </span>
        }
        aside={
          <>
            <SavedBadge visibleSince={savedToken} />
            <OverflowMenu
              label={DETAIL_COPY.itemActions}
              actions={[
                {
                  label: DETAIL_COPY.addFromPurchase,
                  onSelect: addFromPurchase,
                },
                { label: DETAIL_COPY.history, onSelect: showHistory },
                {
                  label: DETAIL_COPY.moveToTrash,
                  onSelect: trash,
                  tone: "danger",
                },
              ]}
            />
          </>
        }
      />

      {showDuplicates && (
        <div className={styles.notice} role="status">
          <p>
            {DETAIL_COPY.duplicatesLead}{" "}
            {duplicates.map((duplicate, index) => (
              <span key={duplicate.id}>
                {index > 0 && DETAIL_COPY.listSeparator}
                <Link to={ROUTES.item(duplicate.id)}>{duplicate.name}</Link>
              </span>
            ))}
          </p>
          <div className={styles.inlineActions}>
            <Button compact onClick={() => setDuplicatesDismissed(true)}>
              {DETAIL_COPY.keepBoth}
            </Button>
            <Button compact variant="ghost" onClick={trash}>
              {DETAIL_COPY.trashThisOne}
            </Button>
          </div>
        </div>
      )}

      <div className={styles.layout}>
        <div className={styles.photoColumn}>
          {item.photos.length > 0 ? (
            <PhotoGallery
              photos={item.photos}
              primaryPhotoId={item.primaryPhotoId}
              itemName={item.name}
              onSetMain={setMainPhoto}
              onRemove={removePhoto}
              addControl={
                <DropZone
                  kind="photo"
                  variant="tile"
                  title={DETAIL_COPY.addPhotos}
                  formats={UPLOAD_COPY.photoFormats}
                  icon={<Camera aria-hidden="true" />}
                  uploads={uploads.uploads}
                  onFiles={uploads.add}
                  onRetry={uploads.retry}
                  onRemove={uploads.remove}
                />
              }
            />
          ) : (
            <DropZone
              kind="photo"
              title={UPLOAD_COPY.addPhoto}
              formats={UPLOAD_COPY.photoFormats}
              icon={<Camera aria-hidden="true" />}
              uploads={uploads.uploads}
              onFiles={uploads.add}
              onRetry={uploads.retry}
              onRemove={uploads.remove}
            />
          )}
        </div>

        <div className={styles.factsColumn}>
          {mode === "edit" ? (
            <ItemEditor
              draft={draft}
              onChange={setDraft}
              errors={errors}
              formError={formError}
              currency={currency}
              busy={busy}
              conflict={conflict}
              reloading={reloading}
              onSave={save}
              onCancel={cancelEdit}
              onReload={reloadAfterConflict}
            />
          ) : (
            <>
              <div className={styles.editRow}>
                <Button
                  variant="outline-accent"
                  compact
                  icon={<Pencil aria-hidden="true" />}
                  onClick={startEdit}
                >
                  {DETAIL_COPY.edit}
                </Button>
              </div>
              <ItemFacts
                item={item}
                currency={currency}
                onEdit={startEdit}
                onSavePrice={savePrice}
              />
            </>
          )}
          <CompatibleParts
            itemId={item.id}
            parts={item.parts}
            onChanged={reload}
          />
          <div ref={historySection} className={styles.historySection}>
            <Disclosure
              key={historyOpenCount}
              summary={DETAIL_COPY.history}
              defaultOpen={historyOpenCount > 0}
            >
              <ItemHistory
                revisions={history.revisions}
                loading={history.loading}
              />
            </Disclosure>
          </div>
        </div>
      </div>

      <Dialog
        open={discardOpen}
        title={DETAIL_COPY.discardTitle}
        onClose={keepEditing}
        footer={
          <>
            <Button onClick={keepEditing}>{DETAIL_COPY.keepEditing}</Button>
            <Button variant="danger" onClick={discard}>
              {DETAIL_COPY.discard}
            </Button>
          </>
        }
      >
        <p>{DETAIL_COPY.discardText}</p>
      </Dialog>
    </div>
  );
}
