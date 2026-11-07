import { useMemo, useState } from "react";
import { Pencil } from "lucide-react";
import { attachmentUrl } from "../../api/client";
import { groupsApi } from "../../api/endpoints";
import type { Group } from "../../api/types";
import { Button } from "../../components/Button";
import { useToast } from "../../components/Toast";
import { formatCosts } from "./costs";
import { ROOM_COPY } from "./copy";
import { RoomDialog } from "./RoomDialog";
import { useRoomCosts } from "./useRoomCosts";
import styles from "./RoomBanner.module.css";

interface RoomBannerProps {
  room: Group;
  /** "1 item", already worded for the current results. */
  countLabel: string;
  onRoomChanged: () => void;
}

/** Heads the Items page when one room is chosen: its photo, words and total. */
export function RoomBanner({
  room,
  countLabel,
  onRoomChanged,
}: RoomBannerProps) {
  const [editing, setEditing] = useState(false);
  const { notify } = useToast();
  const reloadCostsWhenRoomChanges = useMemo(() => [room], [room]);
  const costs = useRoomCosts(true, reloadCostsWhenRoomChanges);
  const total = formatCosts(costs?.rooms[String(room.id)]);
  const facts = total
    ? `${countLabel} · ${ROOM_COPY.total(total)}`
    : countLabel;

  return (
    <section
      className={styles.banner}
      data-has-photo={Boolean(room.photoId) || undefined}
    >
      {room.photoId && (
        <img
          className={styles.photo}
          src={attachmentUrl(room.photoId, "preview")}
          alt=""
        />
      )}
      <div className={styles.content}>
        <div className={styles.words}>
          <h1 className={styles.name}>{room.name}</h1>
          {room.description && (
            <p className={styles.description}>{room.description}</p>
          )}
          <p className={styles.facts} aria-live="polite">
            {facts}
          </p>
        </div>
        <Button
          compact
          className={styles.edit}
          icon={<Pencil aria-hidden="true" />}
          aria-label={ROOM_COPY.editTitle(room.name)}
          onClick={() => setEditing(true)}
        >
          <span className={styles.editLabel}>{ROOM_COPY.editRoom}</span>
        </Button>
      </div>
      {editing && (
        <RoomDialog
          room={room}
          onSubmit={async (name, details) => {
            await groupsApi.update("rooms", room.id, { name, ...details });
            notify({ title: ROOM_COPY.saved(name) });
            onRoomChanged();
          }}
          onClose={() => setEditing(false)}
        />
      )}
    </section>
  );
}
