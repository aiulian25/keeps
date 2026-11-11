import { useId, useState, type FormEvent } from "react";
import { Camera, Info, Upload } from "lucide-react";
import { newIdempotencyKey } from "../../api/client";
import { importsApi } from "../../api/endpoints";
import type { ImportCandidate, ImportDraft, ItemInput } from "../../api/types";
import { Button } from "../../components/Button";
import { FormError } from "../../components/form/Field";
import { PageHeader } from "../../components/shell/PageHeader";
import { COMMON } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import { useSession } from "../../session/SessionContext";
import { MANUAL_COPY } from "../add/copy";
import { useItemSave } from "../add/useItemSave";
import { UPLOAD_COPY } from "../attachments/copy";
import { DropZone } from "../attachments/DropZone";
import { useUploads } from "../attachments/useUploads";
import { ITEM_COPY } from "../items/copy";
import { ItemFields } from "../items/ItemFields";
import { emptyItemInput } from "../items/itemInput";
import { candidatesOf, fieldsFromCandidate } from "./candidateInput";
import { IMPORT_COPY, REVIEW_PRICE_PLACEHOLDER } from "./copy";
import { formatFetchedDate, formatMoney } from "./format";
import { withMainPhotoFirst } from "../attachments/mainPhoto";
import { PhotoPicker } from "./PhotoPicker";
import styles from "./Import.module.css";

interface ReviewImportProps {
  draft: ImportDraft;
  /** The room Add item was opened from, or "". */
  room: string;
  onAddManually: (fields: Partial<ItemInput>) => void;
}

/** Panel 05 with DEV-08: nothing is saved until the user presses Save. */
export function ReviewImport({
  draft,
  room,
  onAddManually,
}: ReviewImportProps) {
  const { account } = useSession();
  const candidates = candidatesOf(draft);
  const [candidateIndex, setCandidateIndex] = useState(0);
  const candidate = candidates[candidateIndex];
  const [input, setInput] = useState<ItemInput>(() => ({
    ...emptyItemInput(),
    ...fieldsFromCandidate(candidates[0]),
    room,
  }));
  const [keptPhotoIds, setKeptPhotoIds] = useState(candidates[0].imageIds);
  const [mainPhotoId, setMainPhotoId] = useState<number | null>(
    candidates[0].imageIds[0] ?? null,
  );
  const [idempotencyKey] = useState(newIdempotencyKey);
  const uploads = useUploads();
  const { saving, formError, fieldErrors, setFieldErrors, save } =
    useItemSave();
  const attachmentIds = withMainPhotoFirst(
    [...keptPhotoIds, ...uploads.attachmentIds],
    mainPhotoId,
  );

  const chooseCandidate = (index: number) => {
    setCandidateIndex(index);
    setInput((current) => ({
      ...current,
      ...fieldsFromCandidate(candidates[index]),
    }));
    setKeptPhotoIds(candidates[index].imageIds);
    setMainPhotoId(candidates[index].imageIds[0] ?? null);
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (input.name.trim() === "") {
      setFieldErrors({ name: MANUAL_COPY.nameRequired });
      return;
    }
    save(() =>
      importsApi.save(
        draft.id,
        candidateIndex,
        { ...input, attachmentIds },
        idempotencyKey,
      ),
    );
  };

  const saveLabel = uploads.busy
    ? MANUAL_COPY.waitingForUploads
    : ITEM_COPY.saveItem;

  return (
    <form className={styles.review} onSubmit={submit} noValidate>
      <PageHeader title={IMPORT_COPY.reviewTitle} backTo={ROUTES.add} />
      <FormError message={formError} />
      {draft.partial && <p className="muted">{IMPORT_COPY.partial}</p>}
      {candidates.length > 1 && (
        <VariantPicker
          candidates={candidates}
          selected={candidateIndex}
          onSelect={chooseCandidate}
        />
      )}
      <div className={styles.columns}>
        <div className={styles.media}>
          <PhotoPicker
            key={candidateIndex}
            photoIds={candidate.imageIds}
            keptIds={keptPhotoIds}
            mainId={mainPhotoId}
            onMainChange={setMainPhotoId}
            onKeptChange={setKeptPhotoIds}
            itemName={input.name}
          />
          <DropZone
            kind="photo"
            variant="tile"
            title={UPLOAD_COPY.addYourPhotos}
            formats={UPLOAD_COPY.photoFormats}
            icon={<Camera aria-hidden="true" />}
            uploads={uploads.uploads}
            onFiles={uploads.add}
            onRetry={uploads.retry}
            onRemove={uploads.remove}
          />
          <DropZone
            kind="invoice"
            variant="tile"
            title={UPLOAD_COPY.uploadInvoice}
            formats={UPLOAD_COPY.documentFormats}
            icon={<Upload aria-hidden="true" />}
            uploads={uploads.uploads}
            onFiles={uploads.add}
            onRetry={uploads.retry}
            onRemove={uploads.remove}
          />
        </div>
        <ItemFields
          value={input}
          onChange={setInput}
          errors={fieldErrors}
          currency={account?.currency ?? ""}
          afterName={<ListingPrice candidate={candidate} draft={draft} />}
          pricePlaceholder={REVIEW_PRICE_PLACEHOLDER}
          detailsOpen={room !== ""}
        />
      </div>
      <div className={styles.actions}>
        <button
          type="button"
          className={styles.textButton}
          onClick={() => onAddManually({ ...input, attachmentIds })}
        >
          {IMPORT_COPY.addManuallyInstead}
        </button>
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
  );
}

/** Read-only: the shop's price is never copied into Price paid. */
function ListingPrice({
  candidate,
  draft,
}: {
  candidate: ImportCandidate;
  draft: ImportDraft;
}) {
  const hintId = useId();
  const price = candidate.listingPrice
    ? formatMoney(candidate.listingPrice)
    : COMMON.unknown;
  const fetched = formatFetchedDate(draft.retrievedAt);
  return (
    <div className={styles.listing}>
      <span className={styles.listingLabel}>{IMPORT_COPY.listingPrice}</span>
      <div className={styles.listingBox} aria-describedby={hintId}>
        <span>{price}</span>
        <span className={styles.badge} title={IMPORT_COPY.listingPriceHint}>
          {IMPORT_COPY.listingPrice}
          <Info aria-hidden="true" />
        </span>
      </div>
      <span id={hintId} className="visually-hidden">
        {IMPORT_COPY.listingPriceHint}
      </span>
      {fetched && (
        <p className="muted small">
          {IMPORT_COPY.listingSource(draft.sourceHost, fetched)}
        </p>
      )}
    </div>
  );
}

interface VariantPickerProps {
  candidates: ImportCandidate[];
  selected: number;
  onSelect: (index: number) => void;
}

function VariantPicker({ candidates, selected, onSelect }: VariantPickerProps) {
  const name = useId();
  return (
    <fieldset className={styles.variants}>
      <legend>{IMPORT_COPY.variantLegend}</legend>
      {candidates.map((candidate, index) => (
        <label key={`${candidate.name}-${index}`} className={styles.variant}>
          <input
            type="radio"
            name={name}
            checked={index === selected}
            onChange={() => onSelect(index)}
          />
          <span className={styles.variantName}>{candidate.name}</span>
          {candidate.listingPrice && (
            <span className="muted">{formatMoney(candidate.listingPrice)}</span>
          )}
        </label>
      ))}
    </fieldset>
  );
}
