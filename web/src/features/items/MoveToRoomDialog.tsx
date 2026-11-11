import { useState } from "react";
import type { ItemSummary } from "../../api/types";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { SelectField } from "../../components/form/SelectField";
import { COMMON } from "../../constants/copy";
import { ITEMS_COPY } from "./itemsCopy";
import { useGroups } from "./useGroups";

interface MoveToRoomDialogProps {
  item: ItemSummary;
  onMove: (item: ItemSummary, room: string) => void;
  onClose: () => void;
}

// Every room by name, plus the item's own room while the list is loading, so
// the picker never shows a room the item isn't in.
function roomOptions(roomNames: string[], currentRoom: string) {
  const names = new Set(roomNames);
  if (currentRoom !== "") {
    names.add(currentRoom);
  }
  const sorted = [...names].sort((first, second) =>
    first.localeCompare(second),
  );
  return [
    { value: "", label: ITEMS_COPY.noRoom },
    ...sorted.map((name) => ({ value: name, label: name })),
  ];
}

/** Card menu → Move to another room: pick a room (or none) for one item. */
export function MoveToRoomDialog({
  item,
  onMove,
  onClose,
}: MoveToRoomDialogProps) {
  const rooms = useGroups("rooms");
  const currentRoom = item.room?.name ?? "";
  const [room, setRoom] = useState(currentRoom);
  const move = () => {
    onMove(item, room);
    onClose();
  };
  return (
    <Dialog
      open
      title={ITEMS_COPY.moveTitle(item.name)}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            variant="primary"
            disabled={room === currentRoom}
            onClick={move}
          >
            {ITEMS_COPY.move}
          </Button>
        </>
      }
    >
      <SelectField
        label={ITEMS_COPY.moveRoomLabel}
        value={room}
        options={roomOptions(
          rooms.map((group) => group.name),
          currentRoom,
        )}
        onValueChange={setRoom}
      />
    </Dialog>
  );
}
