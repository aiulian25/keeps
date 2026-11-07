import { useCallback, useEffect, useId, useState, type FormEvent } from "react";
import { FileText, ImagePlus, ReceiptText } from "lucide-react";
import { Link, useParams } from "react-router";
import { attachmentsApi, itemsApi } from "../../api/endpoints";
import type { Attachment, Item } from "../../api/types";
import { Card, LoadingLine, OverflowMenu } from "../../components/Bits";
import { Button, ButtonLink } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, TextField } from "../../components/form/Field";
import { PageHeader } from "../../components/shell/PageHeader";
import { useToast } from "../../components/Toast";
import { COMMON } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import { UPLOAD_COPY } from "./copy";
import { DOCUMENTS_COPY } from "./documentsCopy";
import { DropZone } from "./DropZone";
import { FileRow } from "./FileRow";
import { errorMessage, useItem } from "./useDocuments";
import { useUploads } from "./useUploads";
import styles from "./Documents.module.css";

/** Panel 10: an item's invoices, manuals and photos, plus its notes. */
export function ItemDocumentsPage() {
  const itemId = Number(useParams().itemId);
  const { item, status, error, reload } = useItem(itemId);

  if (status === "loading") {
    return <LoadingLine />;
  }
  if (status === "missing") {
    return (
      <div className="stack">
        <PageHeader
          title={DOCUMENTS_COPY.title}
          backTo={ROUTES.items}
          backLabel={DOCUMENTS_COPY.backToItems}
        />
        <p className="muted">{DOCUMENTS_COPY.itemNotFound}</p>
        <ButtonLink to={ROUTES.items}>{DOCUMENTS_COPY.backToItems}</ButtonLink>
      </div>
    );
  }
  if (status === "failed" || item === null) {
    return (
      <div className="stack">
        <PageHeader
          title={DOCUMENTS_COPY.title}
          backTo={ROUTES.item(itemId)}
          backLabel={DOCUMENTS_COPY.backToItem}
        />
        <FormError message={error} />
        <Button onClick={reload}>{COMMON.retry}</Button>
      </div>
    );
  }
  return <ItemDocuments item={item} reload={reload} />;
}

function ItemDocuments({
  item,
  reload,
}: {
  item: Item;
  reload: () => Promise<void>;
}) {
  const { notify } = useToast();
  const [actionError, setActionError] = useState("");
  const [renaming, setRenaming] = useState<Attachment | null>(null);
  const [confirmingRemoval, setConfirmingRemoval] = useState<Attachment | null>(
    null,
  );

  // A failed list refresh must not look like a failed link: the file is
  // already on the item, so only the refresh is reported.
  const link = useCallback(
    (attachmentIds: number[]) =>
      itemsApi
        .linkAttachments(item.id, attachmentIds)
        .then(() =>
          reload().catch(() => setActionError(UPLOAD_COPY.refreshFailed)),
        ),
    [item.id, reload],
  );
  const onUploaded = useCallback(
    (attachment: Attachment) => link([attachment.id]),
    [link],
  );
  const uploads = useUploads(onUploaded);
  const { remove: removeUpload } = uploads;

  // Once a new file shows in the lists, its upload entry is no longer needed.
  useEffect(() => {
    const attachedIds = new Set(
      [...item.photos, ...item.documents].map((attachment) => attachment.id),
    );
    uploads.uploads
      .filter(
        (upload) =>
          upload.status === "done" &&
          upload.attachment &&
          attachedIds.has(upload.attachment.id),
      )
      .forEach((upload) => removeUpload(upload.key));
  }, [item, uploads.uploads, removeUpload]);

  const remove = async (attachment: Attachment) => {
    setConfirmingRemoval(null);
    setActionError("");
    try {
      await itemsApi.unlinkAttachment(item.id, attachment.id);
      await reload();
      const undo = {
        label: COMMON.undo,
        onAction: () =>
          link([attachment.id]).catch((caught: unknown) =>
            setActionError(errorMessage(caught)),
          ),
      };
      notify({
        title: DOCUMENTS_COPY.removed(attachment.filename),
        action: attachment.shared ? undefined : undo,
      });
    } catch (caught) {
      setActionError(errorMessage(caught));
    }
  };

  const requestRemoval = (attachment: Attachment) => {
    if (attachment.shared) {
      setConfirmingRemoval(attachment);
      return;
    }
    remove(attachment);
  };

  const menuFor = (attachment: Attachment) => (
    <OverflowMenu
      label={DOCUMENTS_COPY.fileActions(attachment.filename)}
      actions={[
        { label: COMMON.rename, onSelect: () => setRenaming(attachment) },
        {
          label: COMMON.remove,
          onSelect: () => requestRemoval(attachment),
          tone: "danger",
        },
      ]}
    />
  );

  return (
    <div className={styles.page}>
      <PageHeader
        backTo={ROUTES.item(item.id)}
        backLabel={DOCUMENTS_COPY.backToItem}
        title={
          <>
            <span className={styles.titleLead}>
              {DOCUMENTS_COPY.documentsFor}
            </span>
            <span className={styles.titleName}>{item.name}</span>
          </>
        }
      />
      <FormError message={actionError} />

      {item.documents.length === 0 && (
        <p className="muted">{DOCUMENTS_COPY.noDocuments}</p>
      )}
      {item.documents.length > 0 && (
        <ul className={styles.list}>
          {item.documents.map((attachment) => (
            <li key={attachment.id}>
              <FileRow attachment={attachment} actions={menuFor(attachment)} />
            </li>
          ))}
        </ul>
      )}

      {item.photos.length > 0 && (
        <section className={styles.section}>
          <h2>{DOCUMENTS_COPY.photos}</h2>
          <ul className={styles.list}>
            {item.photos.map((attachment) => (
              <li key={attachment.id}>
                <FileRow
                  attachment={attachment}
                  actions={menuFor(attachment)}
                />
              </li>
            ))}
          </ul>
        </section>
      )}

      <section className={styles.section}>
        <h2>{DOCUMENTS_COPY.addMore}</h2>
        <div className={styles.tiles}>
          <DropZone
            kind="photo"
            variant="tile"
            title={UPLOAD_COPY.uploadPhotos}
            formats={UPLOAD_COPY.photoFormats}
            icon={<ImagePlus aria-hidden="true" />}
            uploads={uploads.uploads}
            onFiles={uploads.add}
            onRetry={uploads.retry}
            onRemove={uploads.remove}
          />
          <DropZone
            kind="invoice"
            variant="tile"
            title={UPLOAD_COPY.uploadReceipt}
            formats={UPLOAD_COPY.receiptHint}
            icon={<ReceiptText aria-hidden="true" />}
            uploads={uploads.uploads}
            onFiles={uploads.add}
            onRetry={uploads.retry}
            onRemove={uploads.remove}
          />
          <DropZone
            kind="document"
            variant="tile"
            title={UPLOAD_COPY.uploadOtherDocument}
            formats={UPLOAD_COPY.documentFormats}
            icon={<FileText aria-hidden="true" />}
            uploads={uploads.uploads}
            onFiles={uploads.add}
            onRetry={uploads.retry}
            onRemove={uploads.remove}
          />
        </div>
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h2>{DOCUMENTS_COPY.notes}</h2>
          <Link to={ROUTES.item(item.id)} className={styles.textLink}>
            {DOCUMENTS_COPY.editNotes}
          </Link>
        </div>
        <Card className={styles.notes}>
          {item.notes && <p>{item.notes}</p>}
          {!item.notes && <p className="muted">{DOCUMENTS_COPY.noNotes}</p>}
        </Card>
      </section>

      {renaming && (
        <RenameDialog
          attachment={renaming}
          onClose={() => setRenaming(null)}
          onRenamed={() => {
            setRenaming(null);
            reload();
          }}
        />
      )}
      <Dialog
        open={confirmingRemoval !== null}
        title={DOCUMENTS_COPY.removeTitle}
        onClose={() => setConfirmingRemoval(null)}
        footer={
          <>
            <Button onClick={() => setConfirmingRemoval(null)}>
              {COMMON.cancel}
            </Button>
            <Button
              variant="danger"
              onClick={() => confirmingRemoval && remove(confirmingRemoval)}
            >
              {COMMON.remove}
            </Button>
          </>
        }
      >
        <p>{sharedWarning(item.sharedWith.length)}</p>
      </Dialog>
    </div>
  );
}

function sharedWarning(otherItems: number) {
  if (otherItems === 0) {
    return DOCUMENTS_COPY.sharedWarningUnknownCount;
  }
  return DOCUMENTS_COPY.sharedWarning(otherItems);
}

interface RenameDialogProps {
  attachment: Attachment;
  onClose: () => void;
  onRenamed: () => void;
}

function RenameDialog({ attachment, onClose, onRenamed }: RenameDialogProps) {
  const formId = useId();
  const [filename, setFilename] = useState(attachment.filename);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const trimmed = filename.trim();
    if (trimmed === "") {
      return;
    }
    setBusy(true);
    setError("");
    try {
      await attachmentsApi.rename(attachment.id, trimmed);
      onRenamed();
    } catch (caught) {
      setError(errorMessage(caught));
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      title={DOCUMENTS_COPY.renameTitle}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={COMMON.saving}
            disabled={filename.trim() === ""}
          >
            {COMMON.save}
          </Button>
        </>
      }
    >
      <form id={formId} className="stack" onSubmit={submit}>
        <FormError message={error} />
        <TextField
          label={DOCUMENTS_COPY.fileName}
          value={filename}
          onValueChange={setFilename}
        />
      </form>
    </Dialog>
  );
}
