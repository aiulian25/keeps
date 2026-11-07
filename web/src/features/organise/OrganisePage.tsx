import { useState } from "react";
import { Plus } from "lucide-react";
import { Navigate, NavLink, useParams } from "react-router";
import type { Group, GroupKind } from "../../api/types";
import { LoadingLine } from "../../components/Bits";
import { Button } from "../../components/Button";
import { FormError } from "../../components/form/Field";
import { PageHeader } from "../../components/shell/PageHeader";
import { useToast } from "../../components/Toast";
import { COMMON } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import { NameDialog } from "../account/NameDialog";
import { GROUP_KINDS, KIND_ICONS, KINDS_WITH_MOVE } from "./constants";
import { KIND_COPY, ORGANISE_COPY, ROOM_COPY } from "./copy";
import { DeleteGroupDialog } from "./DeleteGroupDialog";
import {
  GroupCard,
  GroupListRow,
  GroupWideRow,
  readSavedView,
  saveView,
  ViewSwitch,
  type GroupView,
} from "./GroupViews";
import { CostSummaryLine } from "./CostSummaryLine";
import { RoomDialog } from "./RoomDialog";
import { useRoomCosts } from "./useRoomCosts";
import { useGroups } from "./useGroups";
import styles from "./Organise.module.css";

function isGroupKind(value: string | undefined): value is GroupKind {
  return GROUP_KINDS.some((kind) => kind === value);
}

/** Tabs with icons (panel 09); LinkTabs has no icon slot. */
function KindTabs() {
  return (
    <nav aria-label={ORGANISE_COPY.tabsLabel} className={styles.tabs}>
      {GROUP_KINDS.map((kind) => {
        const Icon = KIND_ICONS[kind];
        return (
          <NavLink
            key={kind}
            to={ROUTES.organise(kind)}
            className={({ isActive }) =>
              [styles.tab, isActive ? styles.tabActive : ""].join(" ")
            }
          >
            <Icon aria-hidden="true" />
            {KIND_COPY[kind].title}
          </NavLink>
        );
      })}
    </nav>
  );
}

export function OrganisePage() {
  const { kind } = useParams();
  if (!isGroupKind(kind)) {
    return <Navigate to={ROUTES.organise()} replace />;
  }
  return <GroupList key={kind} kind={kind} />;
}

function GroupList({ kind }: { kind: GroupKind }) {
  const { groups, loadError, create, rename, update, remove } = useGroups(kind);
  const [view, setView] = useState<GroupView>(readSavedView);
  const [editingRoom, setEditingRoom] = useState<Group | null | "new">(null);
  const isRooms = kind === "rooms";
  const changeView = (next: GroupView) => {
    setView(next);
    saveView(next);
  };
  const startCreate = () =>
    isRooms ? setEditingRoom("new") : setCreating(true);
  const startEdit = (group: Group) =>
    isRooms ? setEditingRoom(group) : setRenaming(group);
  const GroupItem = {
    cards: GroupCard,
    list: GroupListRow,
    wide: GroupWideRow,
  }[view];
  const costs = useRoomCosts(isRooms, groups);
  const { notify } = useToast();
  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState<Group | null>(null);
  const [deleting, setDeleting] = useState<Group | null>(null);
  const copy = KIND_COPY[kind];

  const moveTargets = KINDS_WITH_MOVE.includes(kind)
    ? (groups ?? []).filter((group) => group.id !== deleting?.id)
    : [];

  return (
    <div className={styles.page}>
      <KindTabs />
      <PageHeader
        title={copy.title}
        aside={
          <Button
            variant="primary"
            icon={<Plus aria-hidden="true" />}
            onClick={startCreate}
          >
            {copy.create}
          </Button>
        }
      />
      <div className={styles.toolbar}>
        {costs && <CostSummaryLine costs={costs} />}
        <ViewSwitch value={view} onChange={changeView} />
      </div>
      <FormError message={loadError} />
      {groups === null && !loadError && <LoadingLine />}
      {groups?.length === 0 && <p className={styles.empty}>{copy.empty}</p>}
      {groups && groups.length > 0 && (
        <ul
          className={view === "cards" ? styles.cardGrid : styles.list}
          data-view={view}
        >
          {groups.map((group) => (
            <GroupItem
              key={group.id}
              kind={kind}
              group={group}
              cost={costs?.rooms[String(group.id)]}
              onEdit={() => startEdit(group)}
              onDelete={() => setDeleting(group)}
            />
          ))}
        </ul>
      )}

      <NameDialog
        open={creating}
        title={copy.create}
        label={ORGANISE_COPY.name}
        initialName=""
        submitLabel={ORGANISE_COPY.create}
        busyLabel={COMMON.saving}
        onSubmit={async (name) => {
          await create(name);
          notify({ title: ORGANISE_COPY.created(name) });
        }}
        onClose={() => setCreating(false)}
      />
      <NameDialog
        open={renaming !== null}
        title={ORGANISE_COPY.renameTitle(copy.singular)}
        label={ORGANISE_COPY.name}
        initialName={renaming?.name ?? ""}
        submitLabel={COMMON.save}
        busyLabel={COMMON.saving}
        onSubmit={async (name) => {
          await rename(renaming?.id ?? 0, name);
          notify({ title: ORGANISE_COPY.renamed(name) });
        }}
        onClose={() => setRenaming(null)}
      />
      {editingRoom !== null && (
        <RoomDialog
          room={editingRoom === "new" ? null : editingRoom}
          onSubmit={async (name, details) => {
            if (editingRoom === "new") {
              await create(name, details);
            } else {
              await update(editingRoom.id, { name, ...details });
            }
            notify({ title: ROOM_COPY.saved(name) });
          }}
          onClose={() => setEditingRoom(null)}
        />
      )}
      <DeleteGroupDialog
        group={deleting}
        moveTargets={moveTargets}
        onDelete={async (moveTo) => {
          await remove(deleting?.id ?? 0, moveTo);
          notify({ title: ORGANISE_COPY.deleted(deleting?.name ?? "") });
        }}
        onClose={() => setDeleting(null)}
      />
    </div>
  );
}
