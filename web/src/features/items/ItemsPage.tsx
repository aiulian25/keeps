import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { LayoutGrid, List, Plus, SlidersHorizontal } from "lucide-react";
import { Link } from "react-router";
import type { ItemSummary } from "../../api/types";
import { Button, ButtonLink, IconButton } from "../../components/Button";
import { useToast } from "../../components/Toast";
import { PageHeader } from "../../components/shell/PageHeader";
import { COMMON } from "../../constants/copy";
import { SEARCH_DEBOUNCE_MS } from "../../constants/limits";
import { ROUTES } from "../../constants/routes";
import type { AddItemState } from "../add/manualState";
import { RoomBanner } from "../organise/RoomBanner";
import { FilterPanel } from "./FilterPanel";
import { ItemCollection, ItemSkeleton, type ItemView } from "./ItemCollection";
import { ITEMS_COPY } from "./itemsCopy";
import {
  PARAMETER,
  SORTS,
  useItemsSearchParameters,
  type ItemSort,
} from "./itemsSearchParams";
import { RoomChipRow } from "./RoomChipRow";
import { SearchField } from "./SearchField";
import { useGroups, useUnassignedCount } from "./useGroups";
import { useItemsQuery } from "./useItemsQuery";
import { useTrashItem } from "./useTrashItem";
import styles from "./Items.module.css";

const VIEW_STORAGE_KEY = "keeps:items-view";

const SORT_LABELS: Record<ItemSort, string> = {
  newest: ITEMS_COPY.sortNewest,
  purchased: ITEMS_COPY.sortPurchased,
  name: ITEMS_COPY.sortName,
};

function storedView(): ItemView {
  try {
    return window.localStorage.getItem(VIEW_STORAGE_KEY) === "list"
      ? "list"
      : "grid";
  } catch {
    return "grid";
  }
}

/** Grid or list, remembered per browser (DEV-07). Storage can be unavailable; the choice then lasts the visit. */
function useViewMode() {
  const [view, setView] = useState<ItemView>(storedView);
  const choose = (next: ItemView) => {
    setView(next);
    try {
      window.localStorage.setItem(VIEW_STORAGE_KEY, next);
    } catch {
      // Private mode or blocked storage: keep the in-memory choice.
    }
  };
  return [view, choose] as const;
}

/** Typing updates the field at once and the URL (and so the results) after a short pause. */
function useSearchDraft(search: string, commit: (value: string) => void) {
  const [draft, setDraft] = useState(search);
  const [lastSearch, setLastSearch] = useState(search);
  if (search !== lastSearch) {
    // The URL changed from outside (back button, Clear search): show it.
    setLastSearch(search);
    setDraft(search);
  }
  useEffect(() => {
    if (draft === search) {
      return;
    }
    const timer = window.setTimeout(() => commit(draft), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [draft, search, commit]);
  return [draft, setDraft] as const;
}

interface ResultsProps {
  results: ReturnType<typeof useItemsQuery>;
  view: ItemView;
  search: string;
  hasFilters: boolean;
  onClearSearch: () => void;
  onClearFilters: () => void;
  onTrash: (item: ItemSummary) => void;
  onLoadMore: () => void;
}

function Results({
  results,
  view,
  search,
  hasFilters,
  onClearSearch,
  onClearFilters,
  onTrash,
  onLoadMore,
}: ResultsProps) {
  if (results.loading) {
    return <ItemSkeleton view={view} />;
  }
  if (results.failed) {
    return (
      <div className={styles.message} role="alert">
        <p>{ITEMS_COPY.loadFailed}</p>
        <Button onClick={results.reload}>{COMMON.retry}</Button>
      </div>
    );
  }
  if (results.items.length === 0) {
    const narrowed = search !== "" || hasFilters;
    const noMatchText =
      search === "" ? ITEMS_COPY.noMatches : ITEMS_COPY.noMatchesFor(search);
    return (
      <div className={styles.message}>
        <p>{narrowed ? noMatchText : ITEMS_COPY.nothingYet}</p>
        <div className={styles.messageActions}>
          {search !== "" && (
            <Button onClick={onClearSearch}>{ITEMS_COPY.clearSearch}</Button>
          )}
          {hasFilters && (
            <Button onClick={onClearFilters}>{ITEMS_COPY.clearFilters}</Button>
          )}
          {!narrowed && (
            <ButtonLink
              to={ROUTES.add}
              variant="primary"
              iconAfter={<Plus aria-hidden="true" />}
            >
              {ITEMS_COPY.addItem}
            </ButtonLink>
          )}
        </div>
      </div>
    );
  }
  return (
    <>
      <ItemCollection
        items={results.items}
        view={view}
        onTrash={onTrash}
        onMoved={results.reload}
      />
      {results.hasMore && (
        <div className={styles.loadMore}>
          <Button
            onClick={onLoadMore}
            busy={results.loadingMore}
            busyLabel={ITEMS_COPY.loadingMore}
          >
            {ITEMS_COPY.loadMore}
          </Button>
        </div>
      )}
    </>
  );
}

/** Panel 03 "Your items" with search, filters, sort and grid/list (IMP-014, IMP-021). */
export function ItemsPage() {
  const parameters = useItemsSearchParameters();
  const { query, search, update } = parameters;
  const results = useItemsQuery(query);
  const [roomsVersion, setRoomsVersion] = useState(0);
  const rooms = useGroups("rooms", roomsVersion);
  const selectedRoom = rooms.find(
    (room) => String(room.id) === parameters.selectedRoom,
  );
  const countLabel = results.loading ? "" : ITEMS_COPY.count(results.total);
  const addState: AddItemState = { room: selectedRoom?.name };
  const categories = useGroups("categories");
  const tags = useGroups("tags");
  const collections = useGroups("collections");
  const unassignedCount = useUnassignedCount();
  const [view, setView] = useViewMode();
  const [filtersOpen, setFiltersOpen] = useState(false);
  const filterPanelId = useId();
  const sortId = useId();
  const searchInput = useRef<HTMLInputElement>(null);
  const trashItem = useTrashItem();
  const { notify } = useToast();

  const commitSearch = useCallback(
    (value: string) =>
      update({ [PARAMETER.search]: value.trim() }, { replace: true }),
    [update],
  );
  const [searchDraft, setSearchDraft] = useSearchDraft(search, commitSearch);

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    commitSearch(searchDraft);
  };
  const clearSearch = () => {
    update({ [PARAMETER.search]: "" });
    searchInput.current?.focus();
  };
  const trash = (item: ItemSummary) =>
    trashItem(item.id, {
      onTrashed: results.reload,
      onRestored: results.reload,
    });
  const loadMore = () =>
    results
      .loadMore()
      .catch(() => notify({ title: ITEMS_COPY.loadFailed, tone: "error" }));
  const hasFilters = parameters.activeFilterCount > 0;
  const filterLabel = hasFilters
    ? ITEMS_COPY.filtersActive(parameters.activeFilterCount)
    : ITEMS_COPY.filters;

  return (
    <div className="stack">
      <RoomChipRow
        rooms={rooms}
        unassignedCount={unassignedCount}
        selected={parameters.selectedRoom}
        linkFor={parameters.roomLink}
      />
      {selectedRoom ? (
        <RoomBanner
          room={selectedRoom}
          countLabel={countLabel}
          onRoomChanged={() => setRoomsVersion((version) => version + 1)}
        />
      ) : (
        <PageHeader
          title={ITEMS_COPY.title}
          aside={
            <span className="muted" aria-live="polite">
              {countLabel}
            </span>
          }
        />
      )}
      <div className={styles.searchRow}>
        <form
          role="search"
          className={styles.searchForm}
          onSubmit={submitSearch}
        >
          <SearchField
            value={searchDraft}
            onValueChange={setSearchDraft}
            inputReference={searchInput}
          />
        </form>
        <ButtonLink
          to={ROUTES.add}
          state={addState}
          variant="primary"
          iconAfter={<Plus aria-hidden="true" />}
        >
          {ITEMS_COPY.addItem}
        </ButtonLink>
      </div>
      <div className={styles.toolbar}>
        <Button
          variant="ghost"
          compact
          icon={<SlidersHorizontal aria-hidden="true" />}
          aria-expanded={filtersOpen}
          aria-controls={filterPanelId}
          onClick={() => setFiltersOpen(!filtersOpen)}
        >
          {filterLabel}
        </Button>
        <div className={styles.toolbarEnd}>
          <label htmlFor={sortId} className="visually-hidden">
            {ITEMS_COPY.sortLabel}
          </label>
          <select
            id={sortId}
            className={styles.select}
            value={query.sort}
            onChange={(event) =>
              update({ [PARAMETER.sort]: event.target.value })
            }
          >
            {SORTS.map((sort) => (
              <option key={sort} value={sort}>
                {SORT_LABELS[sort]}
              </option>
            ))}
          </select>
          <div
            className={styles.viewToggle}
            role="group"
            aria-label={ITEMS_COPY.viewLabel}
          >
            <IconButton
              label={ITEMS_COPY.gridView}
              aria-pressed={view === "grid"}
              onClick={() => setView("grid")}
            >
              <LayoutGrid aria-hidden="true" />
            </IconButton>
            <IconButton
              label={ITEMS_COPY.listView}
              aria-pressed={view === "list"}
              onClick={() => setView("list")}
            >
              <List aria-hidden="true" />
            </IconButton>
          </div>
        </div>
      </div>
      {filtersOpen && (
        <FilterPanel
          id={filterPanelId}
          value={parameters.filterValue}
          onChange={(name, value) => update({ [name]: value })}
          onClear={parameters.clearFilters}
          flagValue={parameters.flagValue}
          categories={categories}
          tags={tags}
          collections={collections}
        />
      )}
      <Results
        results={results}
        view={view}
        search={query.q ?? ""}
        hasFilters={hasFilters}
        onClearSearch={clearSearch}
        onClearFilters={parameters.clearFilters}
        onTrash={trash}
        onLoadMore={loadMore}
      />
      <p className={styles.quietLink}>
        <Link to={ROUTES.trash}>{ITEMS_COPY.recentlyDeleted}</Link>
      </p>
    </div>
  );
}
