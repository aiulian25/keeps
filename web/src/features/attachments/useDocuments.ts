import { useCallback, useEffect, useState } from "react";
import { ApiError } from "../../api/client";
import { attachmentsApi, itemsApi } from "../../api/endpoints";
import type { DocumentEntry, Item } from "../../api/types";
import { ERRORS } from "../../constants/copy";
import { SEARCH_DEBOUNCE_MS } from "../../constants/limits";

const STATUS_NOT_FOUND = 404;

export function errorMessage(error: unknown) {
  return error instanceof ApiError ? error.message : ERRORS.generic;
}

/**
 * Documents across all items, filtered by file or item name, one page at a
 * time. A new search starts again from the first page; a failed later page
 * keeps what is already shown and can be retried.
 */
export function useDocumentSearch(query: string) {
  const [documents, setDocuments] = useState<DocumentEntry[] | null>(null);
  const [total, setTotal] = useState(0);
  const [nextCursor, setNextCursor] = useState("");
  const [error, setError] = useState("");
  const [moreError, setMoreError] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    let current = true;
    const timer = window.setTimeout(() => {
      attachmentsApi
        .documents(query.trim())
        .then((page) => {
          if (current) {
            setDocuments(page.documents);
            setTotal(page.total);
            setNextCursor(page.nextCursor ?? "");
            setError("");
            setMoreError("");
          }
        })
        .catch((caught: unknown) => {
          if (current) {
            setError(errorMessage(caught));
          }
        });
    }, SEARCH_DEBOUNCE_MS);
    return () => {
      current = false;
      window.clearTimeout(timer);
    };
  }, [query]);

  const loadMore = () => {
    if (!nextCursor || loadingMore) {
      return;
    }
    setLoadingMore(true);
    setMoreError("");
    attachmentsApi
      .documents(query.trim(), nextCursor)
      .then((page) => {
        setDocuments((loaded) => [...(loaded ?? []), ...page.documents]);
        setTotal(page.total);
        setNextCursor(page.nextCursor ?? "");
      })
      .catch((caught: unknown) => setMoreError(errorMessage(caught)))
      .finally(() => setLoadingMore(false));
  };

  return {
    documents,
    total,
    hasMore: nextCursor !== "",
    loadMore,
    loadingMore,
    moreError,
    error,
  };
}

type ItemStatus = "loading" | "ready" | "missing" | "failed";

export function useItem(itemId: number) {
  const [item, setItem] = useState<Item | null>(null);
  const [status, setStatus] = useState<ItemStatus>("loading");
  const [error, setError] = useState("");
  const reload = useCallback(
    () =>
      itemsApi
        .get(itemId)
        .then((next) => {
          setItem(next);
          setStatus("ready");
        })
        .catch((caught: unknown) => {
          setError(errorMessage(caught));
          if (
            caught instanceof ApiError &&
            caught.status === STATUS_NOT_FOUND
          ) {
            setStatus("missing");
            return;
          }
          // A failed reload keeps showing what was already loaded.
          setStatus((previous) => (previous === "ready" ? "ready" : "failed"));
        }),
    [itemId],
  );
  useEffect(() => {
    reload();
  }, [reload]);
  return { item, status, error, reload };
}
