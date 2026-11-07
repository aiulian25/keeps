import type { ReactNode } from "react";
import { Home } from "lucide-react";
import { Link, useNavigate } from "react-router";
import type { Group } from "../../api/types";
import { OverflowMenu } from "../../components/Bits";
import { ITEMS_COPY } from "./itemsCopy";
import { UNASSIGNED_ROOM } from "./itemsSearchParams";
import styles from "./Items.module.css";

const VISIBLE_ROOM_COUNT = 5;

interface RoomChipRowProps {
  rooms: Group[];
  unassignedCount: number;
  /** "" for All, a room id, or "unassigned". */
  selected: string;
  linkFor: (room: string) => string;
}

interface ChipLinkProps {
  to: string;
  selected: boolean;
  children: ReactNode;
}

function ChipLink({ to, selected, children }: ChipLinkProps) {
  return (
    <Link
      to={to}
      className={styles.roomChip}
      aria-current={selected ? "true" : undefined}
    >
      {children}
    </Link>
  );
}

/** Panel 03 room chips: a filter, never a required step. */
export function RoomChipRow({
  rooms,
  unassignedCount,
  selected,
  linkFor,
}: RoomChipRowProps) {
  const navigate = useNavigate();
  const visibleRooms = rooms.slice(0, VISIBLE_ROOM_COUNT);
  const selectedHidden = rooms.find(
    (room) => String(room.id) === selected && !visibleRooms.includes(room),
  );
  const shownRooms = selectedHidden
    ? [...visibleRooms, selectedHidden]
    : visibleRooms;
  const hasOverflow = rooms.length > VISIBLE_ROOM_COUNT;

  return (
    <nav className={styles.roomBar} aria-label={ITEMS_COPY.roomsLabel}>
      <div className={styles.roomRow}>
        <ChipLink to={linkFor("")} selected={selected === ""}>
          <Home aria-hidden="true" />
          {ITEMS_COPY.allRooms}
        </ChipLink>
        {shownRooms.map((room) => (
          <ChipLink
            key={room.id}
            to={linkFor(String(room.id))}
            selected={String(room.id) === selected}
          >
            {room.name}
          </ChipLink>
        ))}
        {unassignedCount > 0 && (
          <ChipLink
            to={linkFor(UNASSIGNED_ROOM)}
            selected={selected === UNASSIGNED_ROOM}
          >
            {ITEMS_COPY.unassigned}
          </ChipLink>
        )}
      </div>
      {hasOverflow && (
        <OverflowMenu
          label={ITEMS_COPY.moreRooms}
          actions={rooms.map((room) => ({
            label: room.name,
            onSelect: () => navigate(linkFor(String(room.id))),
          }))}
        />
      )}
    </nav>
  );
}
