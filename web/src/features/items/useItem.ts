import { useCallback, useEffect, useState } from "react";
import { ApiError } from "../../api/client";
import { itemsApi } from "../../api/endpoints";
import type { Item, Revision } from "../../api/types";

const STATUS_NOT_FOUND = 404;

type LoadState =
  | { itemId: number; item: Item; problem: null }
  | { itemId: number; item: null; problem: "not-found" | "failed" };

function problemFor(error: unknown) {
  const notFound =
    error instanceof ApiError && error.status === STATUS_NOT_FOUND;
  return notFound ? "not-found" : "failed";
}

/** One item by id. `reload` resolves with the fresh item so callers can compare versions. */
export function useItem(itemId: number) {
  const [state, setState] = useState<LoadState | null>(null);

  useEffect(() => {
    let active = true;
    itemsApi
      .get(itemId)
      .then((item) => active && setState({ itemId, item, problem: null }))
      .catch(
        (error: unknown) =>
          active &&
          setState({ itemId, item: null, problem: problemFor(error) }),
      );
    return () => {
      active = false;
    };
  }, [itemId]);

  const replace = useCallback(
    (item: Item) => setState({ itemId: item.id, item, problem: null }),
    [],
  );

  const reload = useCallback(async () => {
    const item = await itemsApi.get(itemId);
    setState({ itemId, item, problem: null });
    return item;
  }, [itemId]);

  const current = state?.itemId === itemId ? state : null;
  return {
    item: current?.item ?? null,
    loading: current === null,
    problem: current?.problem ?? null,
    replace,
    reload,
  };
}

/** Change history; refetched whenever the item's revision moves on. */
export function useItemHistory(itemId: number, revision: number) {
  const [state, setState] = useState<{
    key: string;
    revisions: Revision[];
  } | null>(null);
  const key = `${itemId}:${revision}`;
  useEffect(() => {
    let active = true;
    itemsApi
      .history(itemId)
      .then((revisions) => active && setState({ key, revisions }))
      .catch(() => active && setState({ key, revisions: [] }));
    return () => {
      active = false;
    };
  }, [itemId, key]);
  const current = state?.key === key ? state : null;
  return { revisions: current?.revisions ?? [], loading: current === null };
}
