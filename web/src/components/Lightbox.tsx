import { useEffect, useRef, type KeyboardEvent } from "react";
import { ChevronLeft, ChevronRight, Download, X } from "lucide-react";
import { COMMON } from "../constants/copy";
import { useSwipeTrack } from "./useSwipeTrack";
import styles from "./Lightbox.module.css";

export interface LightboxPhoto {
  id: number;
  /** Full-size image. */
  source: string;
  /** Smaller image shown while the full one loads. */
  placeholder: string;
  downloadName: string;
}

export const LIGHTBOX_COPY = {
  label: "Photo viewer",
  previous: "Previous photo",
  next: "Next photo",
  position: (current: number, total: number) => `${current} of ${total}`,
  download: "Download original",
} as const;

interface LightboxProps {
  photos: LightboxPhoto[];
  startIndex: number;
  altText: string;
  onClose: () => void;
}

/** Full-screen photo viewer: swipe, arrow buttons, arrow keys, Escape. */
export function Lightbox({
  photos,
  startIndex,
  altText,
  onClose,
}: LightboxProps) {
  const dialog = useRef<HTMLDialogElement>(null);
  const { trackReference, index, goTo } = useSwipeTrack(
    photos.length,
    startIndex,
  );
  const current = photos[index] ?? photos[0];

  useEffect(() => {
    dialog.current?.showModal();
    goTo(startIndex, false);
  }, [goTo, startIndex]);

  const onKeyDown = (event: KeyboardEvent<HTMLDialogElement>) => {
    const directions: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1 };
    const direction = directions[event.key];
    if (direction === undefined) {
      return;
    }
    event.preventDefault();
    goTo(index + direction);
  };
  const several = photos.length > 1;

  return (
    <dialog
      ref={dialog}
      className={styles.lightbox}
      aria-label={LIGHTBOX_COPY.label}
      onClose={onClose}
      onKeyDown={onKeyDown}
    >
      <div className={styles.bar}>
        <span className={styles.position} aria-live="polite">
          {LIGHTBOX_COPY.position(index + 1, photos.length)}
        </span>
        <a
          className={styles.barButton}
          href={current.source}
          download={current.downloadName}
          aria-label={LIGHTBOX_COPY.download}
          title={LIGHTBOX_COPY.download}
        >
          <Download aria-hidden="true" />
        </a>
        <button
          type="button"
          className={styles.barButton}
          onClick={() => dialog.current?.close()}
          aria-label={COMMON.close}
          title={COMMON.close}
          autoFocus
        >
          <X aria-hidden="true" />
        </button>
      </div>
      {/* Scrollable regions must be keyboard-focusable (WCAG 2.1.1); arrow keys then move between photos. */}
      <div
        ref={trackReference}
        className={styles.track}
        role="region"
        aria-label={LIGHTBOX_COPY.label}
        tabIndex={0}
      >
        {photos.map((photo, position) => (
          <figure
            key={photo.id}
            className={styles.slide}
            aria-hidden={position !== index}
          >
            <img
              src={photo.source}
              alt={`${altText} — ${LIGHTBOX_COPY.position(position + 1, photos.length)}`}
              style={{ backgroundImage: `url("${photo.placeholder}")` }}
              loading={Math.abs(position - index) <= 1 ? "eager" : "lazy"}
            />
          </figure>
        ))}
      </div>
      {several && (
        <>
          <button
            type="button"
            className={`${styles.arrow} ${styles.previous}`}
            onClick={() => goTo(index - 1)}
            aria-label={LIGHTBOX_COPY.previous}
          >
            <ChevronLeft aria-hidden="true" />
          </button>
          <button
            type="button"
            className={`${styles.arrow} ${styles.next}`}
            onClick={() => goTo(index + 1)}
            aria-label={LIGHTBOX_COPY.next}
          >
            <ChevronRight aria-hidden="true" />
          </button>
        </>
      )}
    </dialog>
  );
}
