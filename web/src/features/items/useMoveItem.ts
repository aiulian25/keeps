import { useCallback } from "react";
import { itemsApi } from "../../api/endpoints";
import type { ItemSummary } from "../../api/types";
import { useToast } from "../../components/Toast";
import { COMMON } from "../../constants/copy";
import { inputFromItem } from "./itemInput";
import { ITEMS_COPY } from "./itemsCopy";

// Saves the item as it is now with only its room changed. The revision check
// still refuses the save if the item was edited somewhere else in between.
async function saveRoom(itemId: number, room: string) {
  const item = await itemsApi.get(itemId);
  await itemsApi.update(itemId, item.revision, {
    ...inputFromItem(item),
    room,
  });
}

function movedTitle(room: string) {
  if (room === "") {
    return ITEMS_COPY.movedOutOfRoom;
  }
  return ITEMS_COPY.movedTo(room);
}

/** Moves an item to another room ("" for none) and offers Undo in the toast. */
export function useMoveItem() {
  const { notify } = useToast();
  return useCallback(
    async (item: ItemSummary, room: string, onMoved: () => void) => {
      const failed = () =>
        notify({ title: ITEMS_COPY.moveFailed, tone: "error" });
      const previousRoom = item.room?.name ?? "";
      try {
        await saveRoom(item.id, room);
      } catch {
        failed();
        return;
      }
      onMoved();
      notify({
        title: movedTitle(room),
        action: {
          label: COMMON.undo,
          onAction: () => {
            saveRoom(item.id, previousRoom).then(onMoved).catch(failed);
          },
        },
      });
    },
    [notify],
  );
}
