import { useEffect, useState, type FormEvent } from "react";
import { Bookmark as BookmarkIcon, X } from "lucide-react";
import { attachmentsApi } from "../../api/endpoints";
import type { Bookmark } from "../../api/types";
import { errorMessage } from "./useDocuments";
import styles from "./DocumentViewer.module.css";

const LABEL_MAXIMUM_LENGTH = 120;

export const BOOKMARK_COPY = {
  label: "Bookmarks",
  markPage: (page: number) => `Mark page ${page}`,
  nameLabel: "Name for this page",
  namePlaceholder: "e.g. Warranty exclusions",
  save: "Mark",
  goTo: (bookmark: Bookmark) =>
    `Go to page ${bookmark.page}: ${bookmark.label}`,
  remove: (label: string) => `Remove bookmark ${label}`,
  beyondEnd: (page: number) =>
    `Page ${page} isn't in this file any more; remove the bookmark or keep it as a note.`,
} as const;

interface DocumentBookmarksProps {
  attachmentId: number;
  currentPage: number;
  pageCount: number;
  onGoTo: (page: number) => void;
}

/** Named pages of a PDF: mark the page in view, jump back to it later. */
export function DocumentBookmarks({
  attachmentId,
  currentPage,
  pageCount,
  onGoTo,
}: DocumentBookmarksProps) {
  const [bookmarks, setBookmarks] = useState<Bookmark[]>([]);
  const [label, setLabel] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let current = true;
    attachmentsApi
      .bookmarks(attachmentId)
      .then((loaded) => current && setBookmarks(loaded))
      .catch((caught: unknown) => current && setError(errorMessage(caught)));
    return () => {
      current = false;
    };
  }, [attachmentId]);

  const mark = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const added = await attachmentsApi.addBookmark(attachmentId, {
        page: currentPage,
        label,
      });
      setBookmarks((list) =>
        [...list, added].sort((first, second) => first.page - second.page),
      );
      setLabel("");
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (bookmark: Bookmark) => {
    setError("");
    try {
      await attachmentsApi.removeBookmark(attachmentId, bookmark.id);
      setBookmarks((list) => list.filter((item) => item.id !== bookmark.id));
    } catch (caught) {
      setError(errorMessage(caught));
    }
  };

  return (
    <section className={styles.bookmarks} aria-label={BOOKMARK_COPY.label}>
      {bookmarks.length > 0 && (
        <ul className={styles.bookmarkList}>
          {bookmarks.map((bookmark) => {
            const beyondEnd = pageCount > 0 && bookmark.page > pageCount;
            return (
              <li key={bookmark.id} className={styles.bookmark}>
                <button
                  type="button"
                  className={styles.bookmarkJump}
                  onClick={() => onGoTo(bookmark.page)}
                  disabled={beyondEnd}
                  aria-label={BOOKMARK_COPY.goTo(bookmark)}
                  title={
                    beyondEnd
                      ? BOOKMARK_COPY.beyondEnd(bookmark.page)
                      : undefined
                  }
                >
                  <BookmarkIcon aria-hidden="true" />
                  <span>
                    p.{bookmark.page} {bookmark.label}
                  </span>
                </button>
                <button
                  type="button"
                  className={styles.bookmarkRemove}
                  onClick={() => remove(bookmark)}
                  aria-label={BOOKMARK_COPY.remove(bookmark.label)}
                >
                  <X aria-hidden="true" />
                </button>
                {beyondEnd && (
                  <span className="visually-hidden">
                    {BOOKMARK_COPY.beyondEnd(bookmark.page)}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
      <form className={styles.bookmarkForm} onSubmit={mark}>
        <label className="visually-hidden" htmlFor={`bookmark-${attachmentId}`}>
          {BOOKMARK_COPY.nameLabel}
        </label>
        <input
          id={`bookmark-${attachmentId}`}
          className={styles.bookmarkInput}
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          placeholder={BOOKMARK_COPY.namePlaceholder}
          maxLength={LABEL_MAXIMUM_LENGTH}
        />
        <button
          type="submit"
          className={styles.bookmarkSave}
          disabled={busy || label.trim() === ""}
        >
          {BOOKMARK_COPY.markPage(currentPage)}
        </button>
      </form>
      {error && (
        <p className={styles.bookmarkError} role="alert">
          {error}
        </p>
      )}
    </section>
  );
}
