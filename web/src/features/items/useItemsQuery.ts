import { useCallback, useEffect, useState } from "react";
import type { ItemQuery } from "../../api/endpoints";
import { itemsApi } from "../../api/endpoints";
import type { ItemSummary } from "../../api/types";

interface Result {
  key: string;
  items: ItemSummary[];
  total: number;
  nextCursor?: string;
  failed: boolean;
}

function isAbort(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

/**
 * Fetches one query (pass a stable/memoised object). A newer query aborts the
 * request still in flight, so slow responses never overwrite fresh ones.
 */
export function useItemsQuery(query: ItemQuery) {
  const [result, setResult] = useState<Result | null>(null);
  const [reloadCount, setReloadCount] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);
  const key = `${JSON.stringify(query)}#${reloadCount}`;

  useEffect(() => {
    const controller = new AbortController();
    itemsApi
      .list(query, controller.signal)
      .then((page) =>
        setResult({
          key,
          items: page.items,
          total: page.total,
          nextCursor: page.nextCursor,
          failed: false,
        }),
      )
      .catch((error: unknown) => {
        if (isAbort(error)) {
          return;
        }
        setResult({ key, items: [], total: 0, failed: true });
      });
    return () => controller.abort();
  }, [query, key]);

  const current = result?.key === key ? result : null;

  /** Rejects when the next page can't be loaded; the items already shown stay. */
  const loadMore = useCallback(async () => {
    if (!current?.nextCursor) {
      return;
    }
    setLoadingMore(true);
    try {
      const page = await itemsApi.list({
        ...query,
        cursor: current.nextCursor,
      });
      setResult((previous) => {
        if (previous?.key !== key) {
          return previous;
        }
        return {
          ...previous,
          items: [...previous.items, ...page.items],
          nextCursor: page.nextCursor,
        };
      });
    } finally {
      setLoadingMore(false);
    }
  }, [current, query, key]);

  const reload = useCallback(() => setReloadCount((count) => count + 1), []);

  return {
    items: current?.items ?? [],
    total: current?.total ?? 0,
    hasMore: Boolean(current?.nextCursor),
    loading: current === null,
    failed: current?.failed ?? false,
    loadingMore,
    loadMore,
    reload,
  };
}
