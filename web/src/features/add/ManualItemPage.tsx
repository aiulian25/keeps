import { useEffect, useRef, useState, type FormEvent } from "react";
import { Box, Camera, FileText, Layers, ReceiptText } from "lucide-react";
import { useLocation } from "react-router";
import { newIdempotencyKey } from "../../api/client";
import { itemsApi } from "../../api/endpoints";
import type { ItemInput } from "../../api/types";
import { Button } from "../../components/Button";
import { FormError } from "../../components/form/Field";
import { PageHeader } from "../../components/shell/PageHeader";
import { COMMON } from "../../constants/copy";
import { useSession } from "../../session/SessionContext";
import { UPLOAD_COPY } from "../attachments/copy";
import { DropZone } from "../attachments/DropZone";
import { withMainPhotoFirst } from "../attachments/mainPhoto";
import { useUploads } from "../attachments/useUploads";
import { ITEM_COPY } from "../items/copy";
import { ItemFields } from "../items/ItemFields";
import { emptyItemInput } from "../items/itemInput";
import { MANUAL_COPY, MANUAL_DRAFT_STORAGE_KEY, PRESET_COPY } from "./copy";
import type { ManualItemState } from "./manualState";
import {
  clearDraft,
  readManualDraft,
  writeManualDraft,
  type ManualDraft,
} from "./sessionDraft";
import { ROUTES } from "../../constants/routes";
import { useGoBack } from "../../hooks/useGoBack";
import { useItemSave } from "./useItemSave";
import { CapturePresetDialog } from "./CapturePresetDialog";
import styles from "./Add.module.css";

function inputFromHandover(handover: ManualItemState): ItemInput {
  const input: ItemInput = { ...emptyItemInput(), ...handover.fields };
  input.seller = handover.seller || input.seller;
  input.orderReference = handover.orderReference || input.orderReference;
  input.purchaseDate = handover.purchaseDate ?? input.purchaseDate;
  input.purchaseId = handover.purchaseId ?? input.purchaseId;
  if (handover.url && input.notes === "") {
    input.notes = MANUAL_COPY.notesLinkPrefix + handover.url;
  }
  return input;
}

/**
 * Where the form starts. A refresh (the same history entry) resumes its own
 * draft, so the hand-over that opened it is used only once. A new hand-over
 * starts fresh; a visit without one picks up a draft left from earlier.
 */
function startingDraft(entry: string, handover: ManualItemState): ManualDraft {
  const stored = readManualDraft(MANUAL_DRAFT_STORAGE_KEY);
  const handedOver = Object.keys(handover).length > 0;
  const resumable = stored !== null && (stored.entry === entry || !handedOver);
  if (resumable) {
    return {
      ...stored,
      entry,
      idempotencyKey: stored.idempotencyKey || newIdempotencyKey(),
    };
  }
  return {
    entry,
    input: inputFromHandover(handover),
    idempotencyKey: newIdempotencyKey(),
    uploads: [],
    unfinishedUploads: 0,
    mainPhotoId: null,
  };
}

/** Panel 07: only the name is required; files upload while you type. */
export function ManualItemPage() {
  const leave = useGoBack(ROUTES.items);
  const location = useLocation();
  const handover = (location.state ?? {}) as ManualItemState;
  const { account } = useSession();
  const [start] = useState(() => startingDraft(location.key, handover));
  const [input, setInput] = useState<ItemInput>(() => ({
    ...emptyItemInput(),
    ...start.input,
  }));
  const uploads = useUploads(undefined, start.uploads);
  const [presetsOpen, setPresetsOpen] = useState(false);
  const [chosenMainPhotoId, setChosenMainPhotoId] = useState<number | null>(
    start.mainPhotoId,
  );
  const uploadedPhotoIds = uploads.uploads.flatMap((upload) =>
    upload.kind === "photo" && upload.attachment ? [upload.attachment.id] : [],
  );
  const mainPhotoId = uploadedPhotoIds.includes(chosenMainPhotoId ?? -1)
    ? chosenMainPhotoId
    : (uploadedPhotoIds[0] ?? null);
  const { saving, formError, fieldErrors, setFieldErrors, save } =
    useItemSave();
  const invoiceZone = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const finished = uploads.uploads.flatMap((upload) =>
      upload.status === "done" && upload.attachment ? [upload.attachment] : [],
    );
    writeManualDraft(MANUAL_DRAFT_STORAGE_KEY, {
      entry: start.entry,
      input,
      idempotencyKey: start.idempotencyKey,
      uploads: finished,
      unfinishedUploads: uploads.uploads.length - finished.length,
      mainPhotoId: chosenMainPhotoId,
    });
  }, [start, input, uploads.uploads, chosenMainPhotoId]);

  useEffect(() => {
    if (handover.focusInvoice) {
      invoiceZone.current?.querySelector("input")?.focus();
    }
    // Focus is placed once, on arrival.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (input.name.trim() === "") {
      setFieldErrors({ name: MANUAL_COPY.nameRequired });
      return;
    }
    const attachmentIds = withMainPhotoFirst(
      [...input.attachmentIds, ...uploads.attachmentIds],
      mainPhotoId,
    );
    save(
      () => itemsApi.create({ ...input, attachmentIds }, start.idempotencyKey),
      () => clearDraft(MANUAL_DRAFT_STORAGE_KEY),
    );
  };

  const saveLabel = uploads.busy
    ? MANUAL_COPY.waitingForUploads
    : ITEM_COPY.saveItem;
  const carriedFiles = input.attachmentIds.length;

  return (
    <>
      <form className={styles.manual} onSubmit={submit} noValidate>
        <PageHeader
          title={MANUAL_COPY.title}
          icon={<Box aria-hidden="true" />}
        />
        <FormError message={formError} />
        {(handover.url || handover.purchaseItemName || carriedFiles > 0) && (
          <div className={styles.notes}>
            {handover.url && (
              <p className="muted small">
                {MANUAL_COPY.linkKept(handover.url)}
              </p>
            )}
            {handover.purchaseItemName && (
              <p className="muted small">
                {MANUAL_COPY.samePurchase(handover.purchaseItemName)}{" "}
                {MANUAL_COPY.sharedPurchaseFacts}
              </p>
            )}
            {carriedFiles > 0 && (
              <p className="muted small">
                {MANUAL_COPY.carriedFiles(carriedFiles)}
              </p>
            )}
          </div>
        )}
        {start.unfinishedUploads > 0 && (
          <p className="muted small">
            {MANUAL_COPY.chooseAgain(start.unfinishedUploads)}
          </p>
        )}
        <ItemFields
          value={input}
          onChange={setInput}
          errors={fieldErrors}
          currency={account?.currency ?? ""}
          autoFocusName={!handover.focusInvoice}
          detailsStart={
            <div>
              <Button
                compact
                icon={<Layers aria-hidden="true" />}
                onClick={() => setPresetsOpen(true)}
              >
                {PRESET_COPY.use}
              </Button>
            </div>
          }
          uploads={
            <div className={styles.zones}>
              <DropZone
                kind="photo"
                title={UPLOAD_COPY.addPhoto}
                formats={UPLOAD_COPY.photoFormats}
                icon={<Camera aria-hidden="true" />}
                uploads={uploads.uploads}
                onFiles={uploads.add}
                onRetry={uploads.retry}
                onRemove={uploads.remove}
                mainPhotoId={mainPhotoId}
                onMakeMain={setChosenMainPhotoId}
              />
              <div ref={invoiceZone} className={styles.zoneSlot}>
                <DropZone
                  kind="invoice"
                  title={UPLOAD_COPY.addInvoice}
                  formats={UPLOAD_COPY.documentFormats}
                  icon={<ReceiptText aria-hidden="true" />}
                  uploads={uploads.uploads}
                  onFiles={uploads.add}
                  onRetry={uploads.retry}
                  onRemove={uploads.remove}
                />
              </div>
              <DropZone
                kind="document"
                title={UPLOAD_COPY.addDocument}
                formats={UPLOAD_COPY.documentFormats}
                icon={<FileText aria-hidden="true" />}
                uploads={uploads.uploads}
                onFiles={uploads.add}
                onRetry={uploads.retry}
                onRemove={uploads.remove}
              />
            </div>
          }
        />
        <div className={styles.actions}>
          <Button
            variant="ghost"
            onClick={() => {
              clearDraft(MANUAL_DRAFT_STORAGE_KEY);
              leave();
            }}
          >
            {COMMON.cancel}
          </Button>
          <Button
            type="submit"
            variant="primary"
            busy={saving}
            busyLabel={COMMON.saving}
            disabled={uploads.busy}
          >
            {saveLabel}
          </Button>
        </div>
      </form>
      <CapturePresetDialog
        open={presetsOpen}
        input={input}
        onApply={setInput}
        onClose={() => setPresetsOpen(false)}
      />
    </>
  );
}
