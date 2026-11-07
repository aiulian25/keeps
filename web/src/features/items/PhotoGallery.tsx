import { useState, type KeyboardEvent, type ReactNode } from "react";
import { ChevronLeft, ChevronRight, Maximize2, Star } from "lucide-react";
import { attachmentUrl } from "../../api/client";
import type { Attachment } from "../../api/types";
import { OverflowMenu } from "../../components/Bits";
import { Lightbox, LIGHTBOX_COPY } from "../../components/Lightbox";
import { useSwipeTrack } from "../../components/useSwipeTrack";
import { MAIN_PHOTO_COPY } from "../attachments/copy";
import { DETAIL_COPY } from "./itemsCopy";
import styles from "./ItemDetail.module.css";

interface PhotoGalleryProps {
  photos: Attachment[];
  primaryPhotoId: number;
  itemName: string;
  onSetMain: (photo: Attachment) => void;
  onRemove: (photo: Attachment) => void;
  /** Control for adding more photos, shown under the strip. */
  addControl?: ReactNode;
}

function mainPhotoFirst(photos: Attachment[], primaryPhotoId: number) {
  const main = photos.filter((photo) => photo.id === primaryPhotoId);
  const others = photos.filter((photo) => photo.id !== primaryPhotoId);
  return [...main, ...others];
}

function displayVariant(photo: Attachment) {
  return photo.hasPreview ? "preview" : "original";
}

/**
 * Panel 08 photo area: swipe or use ‹ › to browse, tap a photo to view it
 * full size, thumbnails jump straight to a photo.
 */
export function PhotoGallery({
  photos,
  primaryPhotoId,
  itemName,
  onSetMain,
  onRemove,
  addControl,
}: PhotoGalleryProps) {
  const ordered = mainPhotoFirst(photos, primaryPhotoId);
  const { trackReference, index, goTo } = useSwipeTrack(ordered.length);
  const [viewerStart, setViewerStart] = useState<number | null>(null);
  const selected = ordered[Math.min(index, ordered.length - 1)];
  if (!selected) {
    return null;
  }

  const onPhotoKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    const directions: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1 };
    const direction = directions[event.key];
    if (direction === undefined) {
      return;
    }
    event.preventDefault();
    goTo(index + direction);
    const slides =
      trackReference.current?.querySelectorAll<HTMLButtonElement>("button");
    slides?.[(index + direction + ordered.length) % ordered.length]?.focus({
      preventScroll: true,
    });
  };
  const isMain = selected.id === primaryPhotoId;
  const actions = [
    {
      label: DETAIL_COPY.removePhoto,
      onSelect: () => onRemove(selected),
      tone: "danger" as const,
    },
  ];
  const several = ordered.length > 1;
  const viewerPhotos = ordered.map((photo) => ({
    id: photo.id,
    source: attachmentUrl(photo.id, "original"),
    placeholder: attachmentUrl(photo.id, displayVariant(photo)),
    downloadName: photo.filename,
  }));

  return (
    <div
      className={styles.gallery}
      role="group"
      aria-label={DETAIL_COPY.photosLabel}
    >
      <div className={styles.mainPhotoFrame}>
        {/* The track scrolls horizontally: swipe on touch screens; arrow keys work on the focused photo. */}
        <div ref={trackReference} className={styles.photoTrack}>
          {ordered.map((photo, position) => (
            <button
              key={photo.id}
              type="button"
              className={styles.photoSlide}
              onClick={() => setViewerStart(position)}
              onKeyDown={onPhotoKeyDown}
              aria-label={`${DETAIL_COPY.viewFullSize} — ${DETAIL_COPY.photoNumber(position + 1, ordered.length)}`}
              tabIndex={position === index ? 0 : -1}
            >
              <img
                className={styles.mainPhoto}
                src={attachmentUrl(photo.id, displayVariant(photo))}
                alt={itemName}
                loading={position === 0 ? "eager" : "lazy"}
              />
            </button>
          ))}
        </div>
        <div className={styles.photoMenu}>
          <OverflowMenu label={DETAIL_COPY.photoActions} actions={actions} />
        </div>
        <span className={styles.fullSizeHint} aria-hidden="true">
          <Maximize2 />
        </span>
        {several && isMain && (
          <span className={styles.mainBadge}>
            <Star aria-hidden="true" />
            {MAIN_PHOTO_COPY.mainPhoto}
          </span>
        )}
        {several && !isMain && (
          <button
            type="button"
            className={styles.useAsMain}
            onClick={() => {
              onSetMain(selected);
              goTo(0, false);
            }}
          >
            <Star aria-hidden="true" />
            {MAIN_PHOTO_COPY.useAsMain}
          </button>
        )}
        {several && (
          <>
            <button
              type="button"
              className={`${styles.photoArrow} ${styles.photoArrowPrevious}`}
              onClick={() => goTo(index - 1)}
              aria-label={DETAIL_COPY.previousPhoto}
            >
              <ChevronLeft aria-hidden="true" />
            </button>
            <button
              type="button"
              className={`${styles.photoArrow} ${styles.photoArrowNext}`}
              onClick={() => goTo(index + 1)}
              aria-label={DETAIL_COPY.nextPhoto}
            >
              <ChevronRight aria-hidden="true" />
            </button>
            <span className={styles.photoCounter} aria-live="polite">
              {LIGHTBOX_COPY.position(index + 1, ordered.length)}
            </span>
          </>
        )}
      </div>
      {several && (
        <ul className={styles.thumbnails}>
          {ordered.map((photo, position) => (
            <li key={photo.id}>
              <button
                type="button"
                className={styles.thumbnail}
                aria-label={DETAIL_COPY.photoNumber(
                  position + 1,
                  ordered.length,
                )}
                aria-current={position === index ? "true" : undefined}
                onClick={() => goTo(position)}
              >
                <img src={attachmentUrl(photo.id, "thumbnail")} alt="" />
                {photo.id === primaryPhotoId && (
                  <Star
                    className={styles.thumbnailStar}
                    aria-label={MAIN_PHOTO_COPY.mainPhoto}
                  />
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
      {addControl}
      {viewerStart !== null && (
        <Lightbox
          photos={viewerPhotos}
          startIndex={viewerStart}
          altText={itemName}
          onClose={() => setViewerStart(null)}
        />
      )}
    </div>
  );
}
