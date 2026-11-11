import {
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import { attachmentUrl } from "../../api/client";
import type { Attachment } from "../../api/types";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { PDF_MEDIA_TYPE } from "./fileFormat";
import {
  buildSharingCopy,
  paintRedacted,
  renderPdfPages,
  renderPhoto,
  SHARING_PAGES_MAXIMUM,
  sharingFilename,
  type Box,
  type SharingPage,
} from "./sharingCopy";
import styles from "./Redaction.module.css";

/** Boxes smaller than this (as a fraction of the page) are treated as clicks. */
const MINIMUM_BOX_FRACTION = 0.01;

export const REDACTION_COPY = {
  title: (filename: string) => `Sharing copy of ${filename}`,
  intro:
    "Drag across anything to hide, like your address or other purchases. The pages below show exactly what the copy will contain.",
  safety:
    "The copy is a picture of each page: covered parts are gone for good, and its text can't be selected or searched. Your original stays as it is.",
  loading: "Preparing the pages…",
  failed: "This file couldn't be opened for a sharing copy.",
  truncated: (shown: number, total: number) =>
    `Only the first ${shown} of ${total} pages can go into a sharing copy.`,
  page: (number: number) => `Page ${number}`,
  include: (number: number) => `Include page ${number}`,
  boxes: (count: number) =>
    count === 1 ? "1 area covered" : `${count} areas covered`,
  undo: "Uncover the last area",
  download: "Download sharing copy",
  noPages: "Choose at least one page.",
  pointerOnly:
    "Covering areas needs a mouse, pen or touch. Leave out a whole page instead by clearing its checkbox.",
} as const;

function normalised(
  event: ReactPointerEvent<HTMLCanvasElement>,
  canvas: HTMLCanvasElement,
) {
  const bounds = canvas.getBoundingClientRect();
  return {
    x: Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width)),
    y: Math.min(1, Math.max(0, (event.clientY - bounds.top) / bounds.height)),
  };
}

function boxBetween(
  start: { x: number; y: number },
  end: { x: number; y: number },
): Box {
  return {
    x: Math.min(start.x, end.x),
    y: Math.min(start.y, end.y),
    width: Math.abs(end.x - start.x),
    height: Math.abs(end.y - start.y),
  };
}

function RedactablePage({
  page,
  number,
  locked,
  onChange,
  onDrawingChange,
}: {
  page: SharingPage;
  number: number;
  locked: boolean;
  onChange: (page: SharingPage) => void;
  onDrawingChange: (drawing: boolean) => void;
}) {
  const preview = useRef<HTMLCanvasElement>(null);
  const [start, setStart] = useState<{ x: number; y: number } | null>(null);
  const [draft, setDraft] = useState<Box | null>(null);

  useEffect(() => {
    if (preview.current) {
      paintRedacted(
        preview.current,
        page.canvas,
        draft ? [...page.boxes, draft] : page.boxes,
      );
    }
  }, [page, draft]);

  const begin = (event: ReactPointerEvent<HTMLCanvasElement>) => {
    if (locked) {
      return;
    }
    // Capture keeps the drag going outside the canvas; some pointers can't be
    // captured, and the drag still works without it.
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // continue uncaptured
    }
    setStart(normalised(event, event.currentTarget));
    onDrawingChange(true);
  };
  // An interrupted drag (cancelled, or capture lost) is dropped visibly, so
  // the preview never shows a cover the copy won't have.
  const abandon = () => {
    if (!start) {
      return;
    }
    setStart(null);
    setDraft(null);
    onDrawingChange(false);
  };
  const move = (event: ReactPointerEvent<HTMLCanvasElement>) => {
    if (start) {
      setDraft(boxBetween(start, normalised(event, event.currentTarget)));
    }
  };
  const end = (event: ReactPointerEvent<HTMLCanvasElement>) => {
    if (!start) {
      return;
    }
    const box = boxBetween(start, normalised(event, event.currentTarget));
    setStart(null);
    setDraft(null);
    onDrawingChange(false);
    if (box.width > MINIMUM_BOX_FRACTION && box.height > MINIMUM_BOX_FRACTION) {
      onChange({ ...page, boxes: [...page.boxes, box] });
    }
  };

  return (
    <li className={styles.page} data-included={page.include || undefined}>
      <div className={styles.pageBar}>
        <label className={styles.include}>
          <input
            type="checkbox"
            checked={page.include}
            onChange={(event) =>
              onChange({ ...page, include: event.target.checked })
            }
          />
          {REDACTION_COPY.include(number)}
        </label>
        <span className="muted small" aria-live="polite">
          {REDACTION_COPY.boxes(page.boxes.length)}
        </span>
        {page.boxes.length > 0 && (
          <Button
            compact
            variant="ghost"
            onClick={() =>
              onChange({ ...page, boxes: page.boxes.slice(0, -1) })
            }
          >
            {REDACTION_COPY.undo}
          </Button>
        )}
      </div>
      <canvas
        ref={preview}
        className={styles.canvas}
        role="img"
        aria-label={REDACTION_COPY.page(number)}
        onPointerDown={begin}
        onPointerMove={move}
        onPointerUp={end}
        onPointerCancel={abandon}
        onLostPointerCapture={abandon}
      />
    </li>
  );
}

/**
 * Document or photo → Create sharing copy: choose pages, cover areas, check
 * the preview, download. Nothing is saved or sent; the original is untouched.
 */
export function RedactionDialog({
  attachment,
  onClose,
}: {
  attachment: Attachment;
  onClose: () => void;
}) {
  const [pages, setPages] = useState<SharingPage[] | null>(null);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [drawingPage, setDrawingPage] = useState<number | null>(null);
  const isPdf = attachment.mediaType === PDF_MEDIA_TYPE;

  useEffect(() => {
    let current = true;
    fetch(attachmentUrl(attachment.id), { credentials: "same-origin" })
      .then((response) => {
        if (!response.ok) {
          throw new Error(String(response.status));
        }
        return response.blob();
      })
      .then(async (content) => {
        if (isPdf) {
          const rendered = await renderPdfPages(await content.arrayBuffer());
          return { canvases: rendered.pages, total: rendered.total };
        }
        return { canvases: [await renderPhoto(content)], total: 1 };
      })
      .then(({ canvases, total: count }) => {
        if (current) {
          setPages(
            canvases.map((canvas) => ({ canvas, include: true, boxes: [] })),
          );
          setTotal(count);
        }
      })
      .catch(() => current && setError(REDACTION_COPY.failed));
    return () => {
      current = false;
    };
  }, [attachment.id, isPdf]);

  const update = (index: number, page: SharingPage) =>
    setPages(
      (list) => list?.map((item, at) => (at === index ? page : item)) ?? list,
    );

  const download = async () => {
    if (!pages?.some((page) => page.include)) {
      setError(REDACTION_COPY.noPages);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const blob = await buildSharingCopy(pages, isPdf);
      const link = document.createElement("a");
      link.href = URL.createObjectURL(blob);
      link.download = sharingFilename(attachment.filename, isPdf);
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(link.href);
    } catch {
      setError(REDACTION_COPY.failed);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      title={REDACTION_COPY.title(attachment.filename)}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            variant="primary"
            busy={busy}
            onClick={download}
            disabled={pages === null || drawingPage !== null}
          >
            {REDACTION_COPY.download}
          </Button>
        </>
      }
    >
      <div className={styles.body}>
        <p>{REDACTION_COPY.intro}</p>
        <p className="muted small">{REDACTION_COPY.safety}</p>
        <p className="visually-hidden">{REDACTION_COPY.pointerOnly}</p>
        <FormError message={error} />
        {pages === null && !error && (
          <p className="muted">{REDACTION_COPY.loading}</p>
        )}
        {total > SHARING_PAGES_MAXIMUM && (
          <p className="muted small">
            {REDACTION_COPY.truncated(SHARING_PAGES_MAXIMUM, total)}
          </p>
        )}
        {pages && (
          <ol className={styles.pages}>
            {pages.map((page, index) => (
              <RedactablePage
                key={index}
                page={page}
                number={index + 1}
                locked={busy}
                onChange={(next) => update(index, next)}
                onDrawingChange={(drawing) =>
                  setDrawingPage(drawing ? index : null)
                }
              />
            ))}
          </ol>
        )}
      </div>
    </Dialog>
  );
}
