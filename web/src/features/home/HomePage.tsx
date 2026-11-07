import { useRef, useState, type FormEvent } from "react";
import { ArrowRight, Link2, Pencil, Plus } from "lucide-react";
import { Link, useNavigate } from "react-router";
import type { ItemQuery } from "../../api/endpoints";
import type { ItemSummary } from "../../api/types";
import { Button, ButtonLink } from "../../components/Button";
import { FieldShell } from "../../components/form/Field";
import fieldStyles from "../../components/form/Field.module.css";
import { PageHeader } from "../../components/shell/PageHeader";
import { COMMON } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import type { AddItemState } from "../add/manualState";
import { ItemCollection, ItemSkeleton } from "../items/ItemCollection";
import { ITEMS_COPY } from "../items/itemsCopy";
import {
  itemsLinkForRoom,
  itemsLinkForSearch,
} from "../items/itemsSearchParams";
import { RoomChipRow } from "../items/RoomChipRow";
import { SearchField } from "../items/SearchField";
import { useGroups, useUnassignedCount } from "../items/useGroups";
import { useItemsQuery } from "../items/useItemsQuery";
import { useTrashItem } from "../items/useTrashItem";
import { HOME_COPY } from "./copy";
import styles from "./Home.module.css";

const RECENT_COUNT = 8;
const RECENT_QUERY: ItemQuery = { sort: "newest", limit: RECENT_COUNT };

/** UX-03: one obvious next step, no tour. */
function EmptyHome() {
  const navigate = useNavigate();
  const [url, setUrl] = useState("");
  const submit = (event: FormEvent) => {
    event.preventDefault();
    const state: AddItemState = { url: url.trim(), autoStart: true };
    navigate(ROUTES.add, { state });
  };
  return (
    <section className={styles.empty}>
      <h1>{HOME_COPY.emptyTitle}</h1>
      <p className="muted">{HOME_COPY.emptyText}</p>
      <form className={styles.emptyForm} onSubmit={submit}>
        <FieldShell label={HOME_COPY.linkLabel}>
          {({ id }) => (
            <div className={fieldStyles.control}>
              <span className={fieldStyles.prefix}>
                <Link2 aria-hidden="true" />
              </span>
              <input
                id={id}
                className={fieldStyles.input}
                type="url"
                inputMode="url"
                autoComplete="off"
                placeholder={HOME_COPY.linkPlaceholder}
                value={url}
                onChange={(event) => setUrl(event.target.value)}
              />
            </div>
          )}
        </FieldShell>
        <Button
          type="submit"
          variant="primary"
          wide
          disabled={url.trim() === ""}
          iconAfter={<ArrowRight aria-hidden="true" />}
        >
          {HOME_COPY.getDetails}
        </Button>
      </form>
      <p className={styles.or}>{COMMON.or}</p>
      <ButtonLink to={ROUTES.addManual} icon={<Pencil aria-hidden="true" />}>
        {HOME_COPY.addManually}
      </ButtonLink>
    </section>
  );
}

/** OD-03 landing: rooms, search, Add item and the latest items. No stats or widgets. */
export function HomePage() {
  const navigate = useNavigate();
  const recent = useItemsQuery(RECENT_QUERY);
  const rooms = useGroups("rooms");
  const unassignedCount = useUnassignedCount();
  const trashItem = useTrashItem();
  const searchInput = useRef<HTMLInputElement>(null);
  const [search, setSearch] = useState("");

  const householdEmpty =
    !recent.loading && !recent.failed && recent.total === 0;
  if (householdEmpty) {
    return <EmptyHome />;
  }

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    navigate(itemsLinkForSearch(search));
  };
  const trash = (item: ItemSummary) =>
    trashItem(item.id, {
      onTrashed: () => recent.removeItem(item.id),
      onRestored: recent.reload,
    });

  return (
    <div className="stack">
      <RoomChipRow
        rooms={rooms}
        unassignedCount={unassignedCount}
        selected=""
        linkFor={itemsLinkForRoom}
      />
      <PageHeader title={HOME_COPY.title} />
      <div className={styles.searchRow}>
        <form
          role="search"
          className={styles.searchForm}
          onSubmit={submitSearch}
        >
          <SearchField
            value={search}
            onValueChange={setSearch}
            inputReference={searchInput}
          />
        </form>
        <ButtonLink
          to={ROUTES.add}
          variant="primary"
          iconAfter={<Plus aria-hidden="true" />}
        >
          {ITEMS_COPY.addItem}
        </ButtonLink>
      </div>
      <section className="stack" aria-labelledby="recently-added">
        <div className={styles.sectionHeader}>
          <h2 id="recently-added">{HOME_COPY.recentlyAdded}</h2>
          <Link to={ROUTES.items} className={styles.seeAll}>
            {HOME_COPY.seeAll}
          </Link>
        </div>
        <RecentItems recent={recent} onTrash={trash} />
      </section>
    </div>
  );
}

function RecentItems({
  recent,
  onTrash,
}: {
  recent: ReturnType<typeof useItemsQuery>;
  onTrash: (item: ItemSummary) => void;
}) {
  if (recent.loading) {
    return <ItemSkeleton view="grid" count={RECENT_COUNT} />;
  }
  if (recent.failed) {
    return (
      <div className={styles.message} role="alert">
        <p>{HOME_COPY.loadFailed}</p>
        <Button onClick={recent.reload}>{COMMON.retry}</Button>
      </div>
    );
  }
  return <ItemCollection items={recent.items} view="grid" onTrash={onTrash} />;
}
