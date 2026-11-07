import { useCallback } from "react";
import { itemsApi } from "../../api/endpoints";
import { useToast } from "../../components/Toast";
import { COMMON } from "../../constants/copy";
import { ITEMS_COPY } from "./itemsCopy";

interface TrashCallbacks {
  onTrashed: () => void;
  onRestored: () => void;
}

/** Moves an item to Recently deleted and offers Undo in the toast. */
export function useTrashItem() {
  const { notify } = useToast();
  return useCallback(
    async (itemId: number, { onTrashed, onRestored }: TrashCallbacks) => {
      try {
        await itemsApi.trash(itemId);
      } catch {
        notify({ title: ITEMS_COPY.trashFailed, tone: "error" });
        return;
      }
      onTrashed();
      notify({
        title: ITEMS_COPY.movedToTrash,
        action: {
          label: COMMON.undo,
          onAction: () => {
            itemsApi
              .restore(itemId)
              .then(onRestored)
              .catch(() =>
                notify({ title: ITEMS_COPY.restoreFailed, tone: "error" }),
              );
          },
        },
      });
    },
    [notify],
  );
}
