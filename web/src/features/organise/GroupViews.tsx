import { createElement } from "react";
import {
  ChevronRight,
  LayoutGrid,
  List,
  Rows3,
  type LucideIcon,
} from "lucide-react";
import { Link } from "react-router";
import { attachmentUrl } from "../../api/client";
import type { CostSummary, Group, GroupKind } from "../../api/types";
import { formatCosts } from "./costs";
import { OverflowMenu } from "../../components/Bits";
import { COMMON } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import {
  DEFAULT_ROOM_ICON,
  ITEM_FILTER_PARAMETERS,
  KIND_ICONS,
  MAXIMUM_THUMBNAILS,
  ROOM_ICONS,
} from "./constants";
import { ORGANISE_COPY, ROOM_COPY, VIEW_COPY } from "./copy";
import styles from "./Organise.module.css";

export type GroupView = "cards" | "list" | "wide";

const VIEW_STORAGE_KEY = "keeps.organise.view";
const DEFAULT_VIEW: GroupView = "wide";
const VIEWS: { view: GroupView; label: string; icon: LucideIcon }[] = [
  { view: "cards", label: VIEW_COPY.cards, icon: LayoutGrid },
  { view: "list", label: VIEW_COPY.list, icon: List },
  { view: "wide", label: VIEW_COPY.wide, icon: Rows3 },
];

/** The chosen view is a per-browser convenience; it falls back quietly. */
export function readSavedView(): GroupView {
  try {
    const saved = window.localStorage.getItem(VIEW_STORAGE_KEY);
    return VIEWS.some(({ view }) => view === saved)
      ? (saved as GroupView)
      : DEFAULT_VIEW;
  } catch {
    return DEFAULT_VIEW;
  }
}

export function saveView(view: GroupView) {
  try {
    window.localStorage.setItem(VIEW_STORAGE_KEY, view);
  } catch {
    // Storage can be unavailable (private windows); the choice just isn't remembered.
  }
}

export function ViewSwitch({
  value,
  onChange,
}: {
  value: GroupView;
  onChange: (view: GroupView) => void;
}) {
  return (
    <div
      className={styles.viewSwitch}
      role="group"
      aria-label={VIEW_COPY.label}
    >
      {VIEWS.map(({ view, label, icon: Icon }) => (
        <button
          key={view}
          type="button"
          className={styles.viewButton}
          aria-pressed={value === view}
          aria-label={label}
          title={label}
          onClick={() => onChange(view)}
        >
          <Icon aria-hidden="true" />
        </button>
      ))}
    </div>
  );
}

function iconFor(kind: GroupKind, name: string) {
  if (kind !== "rooms") {
    return KIND_ICONS[kind];
  }
  const lowerName = name.toLowerCase();
  return (
    ROOM_ICONS.find(({ match }) => lowerName.includes(match))?.icon ??
    DEFAULT_ROOM_ICON
  );
}

interface GroupViewProps {
  kind: GroupKind;
  group: Group;
  /** Rooms only: what was paid for the room's items. */
  cost?: CostSummary;
  onEdit: () => void;
  onDelete: () => void;
}

function itemsLinkFor(kind: GroupKind, group: Group) {
  return `${ROUTES.items}?${new URLSearchParams({ [ITEM_FILTER_PARAMETERS[kind]]: String(group.id) })}`;
}

/** "8 items · £1247.50" — the cost only when something has a price. */
function CountLine({ group, cost }: { group: Group; cost?: CostSummary }) {
  const formatted = formatCosts(cost);
  return (
    <span className="muted small">
      {ORGANISE_COPY.itemCount(group.itemCount)}
      {formatted && (
        <>
          {" · "}
          <span className={styles.cost}>{formatted}</span>
        </>
      )}
    </span>
  );
}

function GroupMenu({ kind, group, onEdit, onDelete }: GroupViewProps) {
  return (
    <OverflowMenu
      label={ORGANISE_COPY.menuLabel(group.name)}
      actions={[
        {
          label: kind === "rooms" ? ROOM_COPY.edit : COMMON.rename,
          onSelect: onEdit,
        },
        { label: COMMON.delete, onSelect: onDelete, tone: "danger" },
      ]}
    />
  );
}

function Thumbnails({ group }: { group: Group }) {
  const thumbnails = group.thumbnailIds.slice(0, MAXIMUM_THUMBNAILS);
  const hiddenCount = group.itemCount - thumbnails.length;
  return (
    <span className={styles.thumbnails} aria-hidden="true">
      {thumbnails.map((attachmentId) => (
        <img
          key={attachmentId}
          src={attachmentUrl(attachmentId, "thumbnail")}
          alt=""
          loading="lazy"
        />
      ))}
      {hiddenCount > 0 && thumbnails.length > 0 && (
        <span className={styles.moreTile}>
          {ORGANISE_COPY.moreThumbnails(hiddenCount)}
        </span>
      )}
    </span>
  );
}

/** Wide (panel 09): icon, name, count, description, thumbnails, chevron. */
export function GroupWideRow(props: GroupViewProps) {
  const { kind, group } = props;
  return (
    <li className={styles.row}>
      <Link to={itemsLinkFor(kind, group)} className={styles.rowLink}>
        {group.photoId ? (
          <img
            className={styles.rowPhoto}
            src={attachmentUrl(group.photoId, "thumbnail")}
            alt=""
          />
        ) : (
          createElement(iconFor(kind, group.name), {
            className: styles.rowIcon,
            "aria-hidden": true,
          })
        )}
        <span className={styles.rowText}>
          <span className={styles.rowName}>{group.name}</span>
          <CountLine group={group} cost={props.cost} />
          {group.description && (
            <span className={`muted small ${styles.oneLine}`}>
              {group.description}
            </span>
          )}
        </span>
        <Thumbnails group={group} />
        <ChevronRight className={styles.chevron} aria-hidden="true" />
      </Link>
      <GroupMenu {...props} />
    </li>
  );
}

/** List: one compact line per room. */
export function GroupListRow(props: GroupViewProps) {
  const { kind, group } = props;
  return (
    <li className={styles.compactRow}>
      <Link to={itemsLinkFor(kind, group)} className={styles.compactLink}>
        {createElement(iconFor(kind, group.name), {
          className: styles.compactIcon,
          "aria-hidden": true,
        })}
        <span className={styles.rowName}>{group.name}</span>
        {group.description && (
          <span
            className={`muted small ${styles.oneLine} ${styles.compactDescription}`}
          >
            {group.description}
          </span>
        )}
        <CountLine group={group} cost={props.cost} />
        <ChevronRight className={styles.chevron} aria-hidden="true" />
      </Link>
      <GroupMenu {...props} />
    </li>
  );
}

/** Cards: the room photo as cover (or the newest item photo, or the icon). */
export function GroupCard(props: GroupViewProps) {
  const { kind, group } = props;
  const coverId = group.photoId ?? group.thumbnailIds[0];
  return (
    <li className={styles.card}>
      <Link to={itemsLinkFor(kind, group)} className={styles.cardLink}>
        <span className={styles.cardCover}>
          {coverId ? (
            <img
              src={attachmentUrl(coverId, "preview")}
              alt=""
              loading="lazy"
            />
          ) : (
            createElement(iconFor(kind, group.name), { "aria-hidden": true })
          )}
        </span>
        <span className={styles.cardText}>
          <span className={styles.rowName}>{group.name}</span>
          <CountLine group={group} cost={props.cost} />
          {group.description && (
            <span className={`muted small ${styles.twoLines}`}>
              {group.description}
            </span>
          )}
        </span>
      </Link>
      <span className={styles.cardMenu}>
        <GroupMenu {...props} />
      </span>
    </li>
  );
}
