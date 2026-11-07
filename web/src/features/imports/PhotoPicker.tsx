import { useState } from "react";
import { ChevronLeft, ChevronRight, ImageOff, Star } from "lucide-react";
import { attachmentUrl } from "../../api/client";
import { IconButton } from "../../components/Button";
import { MAIN_PHOTO_COPY } from "../attachments/copy";
import { IMPORT_COPY } from "./copy";
import styles from "./Import.module.css";

interface PhotoPickerProps {
  photoIds: number[];
  keptIds: number[];
  onKeptChange: (ids: number[]) => void;
  mainId: number | null;
  onMainChange: (id: number) => void;
  itemName: string;
}

/** The shop's photos, one at a time; each can be left out before saving. */
export function PhotoPicker({
  photoIds,
  keptIds,
  onKeptChange,
  mainId,
  onMainChange,
  itemName,
}: PhotoPickerProps) {
  const [index, setIndex] = useState(0);
  if (photoIds.length === 0) {
    return (
      <div className={styles.noPhoto}>
        <ImageOff aria-hidden="true" />
        <span>{IMPORT_COPY.noPhoto}</span>
      </div>
    );
  }
  const count = photoIds.length;
  const current = photoIds[index % count];
  const kept = keptIds.includes(current);
  const isMain = current === mainId;
  const toggle = () =>
    onKeptChange(
      kept ? keptIds.filter((id) => id !== current) : [...keptIds, current],
    );
  const step = (offset: number) => setIndex((index + offset + count) % count);
  return (
    <div
      className={styles.photoPicker}
      role="group"
      aria-label={IMPORT_COPY.photosLabel}
    >
      <img
        className={styles.photo}
        src={attachmentUrl(current, "preview")}
        alt={IMPORT_COPY.photoAlt(itemName)}
        data-left-out={!kept || undefined}
      />
      <div className={styles.photoControls}>
        {count > 1 && (
          <IconButton
            label={IMPORT_COPY.previousPhoto}
            onClick={() => step(-1)}
          >
            <ChevronLeft aria-hidden="true" />
          </IconButton>
        )}
        <label className={styles.keep}>
          <input type="checkbox" checked={kept} onChange={toggle} />
          {IMPORT_COPY.keepPhoto}
        </label>
        {kept && count > 1 && (
          <button
            type="button"
            className={styles.mainChoice}
            aria-pressed={isMain}
            onClick={() => onMainChange(current)}
          >
            <Star aria-hidden="true" />
            {isMain ? MAIN_PHOTO_COPY.mainPhoto : MAIN_PHOTO_COPY.useAsMain}
          </button>
        )}
        {count > 1 && (
          <IconButton label={IMPORT_COPY.nextPhoto} onClick={() => step(1)}>
            <ChevronRight aria-hidden="true" />
          </IconButton>
        )}
      </div>
      {count > 1 && (
        <p className="muted small" aria-live="polite">
          {IMPORT_COPY.photoPosition((index % count) + 1, count)}
        </p>
      )}
    </div>
  );
}
