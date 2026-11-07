import { useEffect, useRef, useState } from "react";
import { Download, Minus, Plus, X } from "lucide-react";
import type { PDFDocumentProxy } from "pdfjs-dist";
import { attachmentUrl } from "../../api/client";
import type { Attachment } from "../../api/types";
import { COMMON } from "../../constants/copy";
import { PDF_MEDIA_TYPE } from "./fileFormat";
import { DocumentBookmarks } from "./DocumentBookmarks";
import { openPdf } from "./pdfDocument";
import styles from "./DocumentViewer.module.css";

const ZOOM_STEP = 0.25;
const ZOOM_MINIMUM = 0.5;
const ZOOM_MAXIMUM = 3;
const MAXIMUM_PAGES_SHOWN = 100;
const PERCENT = 100;
const QUARTER_VISIBLE = 0.25;
const HALF_VISIBLE = 0.5;
const MOSTLY_VISIBLE = 0.75;
const VISIBILITY_THRESHOLDS = [QUARTER_VISIBLE, HALF_VISIBLE, MOSTLY_VISIBLE];

export const VIEWER_COPY = {
  loading: "Opening the document…",
  failed:
    "This document couldn't be shown. Download it to open it in another app.",
  pages: (count: number) => (count === 1 ? "1 page" : `${count} pages`),
  truncated: (shown: number) =>
    `Showing the first ${shown} pages. Download the file to see the rest.`,
  zoomIn: "Zoom in",
  zoomOut: "Zoom out",
  page: (number: number) => `Page ${number}`,
  noText:
    "This page has no readable text; it may be a scanned image. Download the file to read it in another app.",
} as const;

/** The page's text in reading order, one line per line break in the PDF. */
function pageLines(items: unknown[]) {
  const lines: string[] = [];
  let line = "";
  for (const item of items) {
    const piece = item as { str?: unknown; hasEOL?: unknown };
    if (typeof piece.str === "string") {
      line += piece.str;
    }
    if (piece.hasEOL === true) {
      lines.push(line);
      line = "";
    }
  }
  lines.push(line);
  return lines.map((text) => text.trim()).filter((text) => text !== "");
}

function PdfPage({
  document,
  pageNumber,
  zoom,
  width,
}: {
  document: PDFDocumentProxy;
  pageNumber: number;
  zoom: number;
  width: number;
}) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const [lines, setLines] = useState<string[] | null>(null);
  useEffect(() => {
    let cancelled = false;
    document
      .getPage(pageNumber)
      .then((page) => page.getTextContent())
      .then((content) => !cancelled && setLines(pageLines(content.items)))
      .catch(() => !cancelled && setLines([]));
    return () => {
      cancelled = true;
    };
  }, [document, pageNumber]);
  useEffect(() => {
    let cancelled = false;
    let task: { cancel: () => void } | null = null;
    document.getPage(pageNumber).then((page) => {
      const element = canvas.current;
      if (cancelled || !element || width === 0) {
        return;
      }
      const unscaled = page.getViewport({ scale: 1 });
      const cssScale = (width / unscaled.width) * zoom;
      const pixelRatio = window.devicePixelRatio || 1;
      const viewport = page.getViewport({ scale: cssScale * pixelRatio });
      element.width = Math.floor(viewport.width);
      element.height = Math.floor(viewport.height);
      element.style.width = `${Math.floor(viewport.width / pixelRatio)}px`;
      element.style.height = `${Math.floor(viewport.height / pixelRatio)}px`;
      const render = page.render({ canvas: element, viewport });
      task = render;
      render.promise.catch(() => undefined);
    });
    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [document, pageNumber, zoom, width]);
  // The canvas is what sighted people see; the same page's text follows it
  // as real text, so screen readers can read the receipt itself.
  return (
    <section
      className={styles.pageSection}
      aria-label={VIEWER_COPY.page(pageNumber)}
      data-page={pageNumber}
    >
      <canvas ref={canvas} className={styles.page} aria-hidden="true" />
      {lines !== null && (
        <div className="visually-hidden">
          {lines.length === 0 && <p>{VIEWER_COPY.noText}</p>}
          {lines.map((text, index) => (
            <p key={index}>{text}</p>
          ))}
        </div>
      )}
    </section>
  );
}

function PdfPages({
  bytes,
  zoom,
  onPageCount,
  onVisiblePage,
}: {
  bytes: ArrayBuffer;
  zoom: number;
  onPageCount: (count: number) => void;
  onVisiblePage: (page: number) => void;
}) {
  const [document, setDocument] = useState<PDFDocumentProxy | null>(null);
  const [failed, setFailed] = useState(false);
  const [width, setWidth] = useState(0);
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let destroy = () => undefined as unknown;
    let closed = false;
    openPdf(bytes)
      .then((task) => {
        destroy = () => task.destroy();
        if (closed) {
          destroy();
          return undefined;
        }
        return task.promise;
      })
      .then((loaded) => {
        if (loaded === undefined) {
          return;
        }
        setDocument(loaded);
        onPageCount(loaded.numPages);
      })
      .catch(() => setFailed(true));
    return () => {
      closed = true;
      destroy();
    };
  }, [bytes, onPageCount]);

  useEffect(() => {
    const element = container.current;
    if (!element) {
      return;
    }
    const observer = new ResizeObserver(() => setWidth(element.clientWidth));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  // The page most in view is the one "Mark page" names.
  useEffect(() => {
    const element = container.current;
    if (!element || document === null) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort(
            (first, second) =>
              second.intersectionRatio - first.intersectionRatio,
          )[0];
        const page = Number(
          (visible?.target as HTMLElement | undefined)?.dataset.page,
        );
        if (page > 0) {
          onVisiblePage(page);
        }
      },
      { threshold: VISIBILITY_THRESHOLDS },
    );
    element
      .querySelectorAll("[data-page]")
      .forEach((section) => observer.observe(section));
    return () => observer.disconnect();
  }, [document, onVisiblePage]);

  if (failed) {
    return <p className={styles.message}>{VIEWER_COPY.failed}</p>;
  }
  const shown = Math.min(document?.numPages ?? 0, MAXIMUM_PAGES_SHOWN);
  return (
    <div ref={container} className={styles.pages}>
      {document === null && (
        <p className={styles.message}>{VIEWER_COPY.loading}</p>
      )}
      {document !== null &&
        Array.from({ length: shown }, (_, index) => (
          <PdfPage
            key={index}
            document={document}
            pageNumber={index + 1}
            zoom={zoom}
            width={width}
          />
        ))}
      {document !== null && document.numPages > MAXIMUM_PAGES_SHOWN && (
        <p className={styles.message}>
          {VIEWER_COPY.truncated(MAXIMUM_PAGES_SHOWN)}
        </p>
      )}
    </div>
  );
}

/** Reads a document inside Keeps; Download stays available. */
export function DocumentViewer({
  attachment,
  onClose,
}: {
  attachment: Attachment;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [bytes, setBytes] = useState<ArrayBuffer | null>(null);
  const [failed, setFailed] = useState(false);
  const [zoom, setZoom] = useState(1);
  const [pageCount, setPageCount] = useState(0);
  const [visiblePage, setVisiblePage] = useState(1);
  const isPdf = attachment.mediaType === PDF_MEDIA_TYPE;
  const goToPage = (page: number) =>
    dialog.current
      ?.querySelector(`[data-page="${page}"]`)
      ?.scrollIntoView({ block: "start" });

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  useEffect(() => {
    if (!isPdf) {
      return;
    }
    const controller = new AbortController();
    fetch(attachmentUrl(attachment.id), {
      credentials: "same-origin",
      signal: controller.signal,
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(String(response.status));
        }
        return response.arrayBuffer();
      })
      .then(setBytes)
      .catch((error: unknown) => {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          setFailed(true);
        }
      });
    return () => controller.abort();
  }, [attachment.id, isPdf]);

  const changeZoom = (step: number) =>
    setZoom((current) =>
      Math.min(ZOOM_MAXIMUM, Math.max(ZOOM_MINIMUM, current + step)),
    );

  return (
    <dialog
      ref={dialog}
      className={styles.viewer}
      aria-labelledby="document-viewer-title"
      onClose={onClose}
    >
      <header className={styles.bar}>
        <div className={styles.title}>
          <h2 id="document-viewer-title">{attachment.filename}</h2>
          {pageCount > 0 && (
            <span className="muted small">{VIEWER_COPY.pages(pageCount)}</span>
          )}
        </div>
        {isPdf && (
          <div className={styles.zoom}>
            <button
              type="button"
              className={styles.barButton}
              onClick={() => changeZoom(-ZOOM_STEP)}
              aria-label={VIEWER_COPY.zoomOut}
              title={VIEWER_COPY.zoomOut}
            >
              <Minus aria-hidden="true" />
            </button>
            <span className="small" aria-live="polite">
              {Math.round(zoom * PERCENT)}%
            </span>
            <button
              type="button"
              className={styles.barButton}
              onClick={() => changeZoom(ZOOM_STEP)}
              aria-label={VIEWER_COPY.zoomIn}
              title={VIEWER_COPY.zoomIn}
            >
              <Plus aria-hidden="true" />
            </button>
          </div>
        )}
        <a
          className={styles.barButton}
          href={attachmentUrl(attachment.id)}
          download={attachment.filename}
          aria-label={COMMON.download}
          title={COMMON.download}
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
      </header>
      {isPdf && pageCount > 0 && (
        <DocumentBookmarks
          attachmentId={attachment.id}
          currentPage={visiblePage}
          pageCount={pageCount}
          onGoTo={goToPage}
        />
      )}
      {/* Scrollable regions must be keyboard-focusable (WCAG 2.1.1). */}
      <div
        className={styles.body}
        role="region"
        aria-label={attachment.filename}
        tabIndex={0}
      >
        {failed && <p className={styles.message}>{VIEWER_COPY.failed}</p>}
        {isPdf && bytes !== null && (
          <PdfPages
            bytes={bytes}
            zoom={zoom}
            onPageCount={setPageCount}
            onVisiblePage={setVisiblePage}
          />
        )}
        {isPdf && bytes === null && !failed && (
          <p className={styles.message}>{VIEWER_COPY.loading}</p>
        )}
        {!isPdf && (
          <img
            className={styles.image}
            src={attachmentUrl(attachment.id)}
            alt={attachment.filename}
          />
        )}
      </div>
    </dialog>
  );
}
