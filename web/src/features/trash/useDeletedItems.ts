import { useEffect, useState } from "react";
import { itemsApi } from "../../api/endpoints";
import type { ItemPage, ItemSummary } from "../../api/types";
import { PAGE_SIZE } from "../../constants/limits";
import { errorMessage } from "../account/errors";

const deletedPage = (cursor?: string) =>
  itemsApi.list({ deleted: true, limit: PAGE_SIZE, cursor });

/** Items in the trash, page by page; restore and purge drop them from the list. */
export function useDeletedItems() {
  const [items, setItems] = useState<ItemSummary[] | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [loadError, setLoadError] = useState("");

  const append = (page: ItemPage) => {
    setItems((current) => [...(current ?? []), ...page.items]);
    setNextCursor(page.nextCursor);
    setLoadError("");
  };

  useEffect(() => {
    let current = true;
    deletedPage().then(
      (page) => {
        if (current) {
          setItems(page.items);
          setNextCursor(page.nextCursor);
        }
      },
      (caught) => {
        if (current) {
          setLoadError(errorMessage(caught));
        }
      },
    );
    return () => {
      current = false;
    };
  }, []);

  const drop = (id: number) =>
    setItems((current) => current?.filter((item) => item.id !== id) ?? null);

  return {
    items,
    loadError,
    hasMore: Boolean(nextCursor),
    loadMore: () =>
      deletedPage(nextCursor).then(append, (caught) =>
        setLoadError(errorMessage(caught)),
      ),
    restore: async (id: number) => {
      await itemsApi.restore(id);
      drop(id);
    },
    purge: async (id: number) => {
      await itemsApi.purge(id);
      drop(id);
    },
  };
}
