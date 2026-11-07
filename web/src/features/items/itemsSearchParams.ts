import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";
import type { ItemQuery } from "../../api/endpoints";
import { PAGE_SIZE } from "../../constants/limits";
import { ROUTES } from "../../constants/routes";
import { CONDITION_OPTIONS } from "./copy";

// Every filter lives in the URL so the back button restores it.
export const PARAMETER = {
  search: "q",
  room: "room",
  category: "category",
  tag: "tag",
  collection: "collection",
  hasReceipt: "hasReceipt",
  priceKnown: "priceKnown",
  condition: "condition",
  inWarranty: "inWarranty",
  sort: "sort",
} as const;

export const UNASSIGNED_ROOM = "unassigned";
const FLAG_ON = "1";

export type ItemSort = NonNullable<ItemQuery["sort"]>;
export const SORTS: ItemSort[] = ["newest", "purchased", "name"];
const DEFAULT_SORT: ItemSort = "newest";

type ParameterName = (typeof PARAMETER)[keyof typeof PARAMETER];
const FILTER_PARAMETERS: ParameterName[] = [
  PARAMETER.room,
  PARAMETER.category,
  PARAMETER.tag,
  PARAMETER.collection,
  PARAMETER.hasReceipt,
  PARAMETER.priceKnown,
  PARAMETER.condition,
  PARAMETER.inWarranty,
];

function numberOrUndefined(text: string | null) {
  const value = Number(text);
  if (!text || !Number.isInteger(value) || value <= 0) {
    return undefined;
  }
  return value;
}

function sortFrom(text: string | null): ItemSort {
  const known = SORTS.find((sort) => sort === text);
  return known ?? DEFAULT_SORT;
}

export function queryFromParameters(parameters: URLSearchParams): ItemQuery {
  const room = parameters.get(PARAMETER.room);
  return {
    q: parameters.get(PARAMETER.search)?.trim() || undefined,
    room: numberOrUndefined(room),
    unassigned: room === UNASSIGNED_ROOM,
    category: numberOrUndefined(parameters.get(PARAMETER.category)),
    tag: numberOrUndefined(parameters.get(PARAMETER.tag)),
    collection: numberOrUndefined(parameters.get(PARAMETER.collection)),
    hasReceipt: parameters.get(PARAMETER.hasReceipt) === FLAG_ON,
    priceKnown: parameters.get(PARAMETER.priceKnown) === FLAG_ON,
    condition: CONDITION_OPTIONS.find(
      (option) => option.value === parameters.get(PARAMETER.condition),
    )?.value,
    inWarranty: parameters.get(PARAMETER.inWarranty) === FLAG_ON,
    sort: sortFrom(parameters.get(PARAMETER.sort)),
    limit: PAGE_SIZE,
  };
}

/** Link to the Items page filtered by one room ("" for all rooms). */
export function itemsLinkForRoom(room: string) {
  if (room === "") {
    return ROUTES.items;
  }
  return `${ROUTES.items}?${new URLSearchParams({ [PARAMETER.room]: room })}`;
}

export function itemsLinkForSearch(search: string) {
  const trimmed = search.trim();
  if (trimmed === "") {
    return ROUTES.items;
  }
  return `${ROUTES.items}?${new URLSearchParams({ [PARAMETER.search]: trimmed })}`;
}

/** Reads and writes the Items page's URL state. */
export function useItemsSearchParameters() {
  const [parameters, setParameters] = useSearchParams();
  const parameterText = parameters.toString();
  const query = useMemo(
    () => queryFromParameters(new URLSearchParams(parameterText)),
    [parameterText],
  );

  const update = useCallback(
    (
      changes: Partial<Record<ParameterName, string>>,
      options: { replace?: boolean } = {},
    ) =>
      setParameters(
        (current) => {
          const next = new URLSearchParams(current);
          for (const [name, value] of Object.entries(changes)) {
            if (value) {
              next.set(name, value);
              continue;
            }
            next.delete(name);
          }
          return next;
        },
        { replace: options.replace },
      ),
    [setParameters],
  );

  const clearFilters = useCallback(
    () =>
      update(Object.fromEntries(FILTER_PARAMETERS.map((name) => [name, ""]))),
    [update],
  );

  const roomLink = useCallback(
    (room: string) => {
      const next = new URLSearchParams(parameterText);
      if (room === "") {
        next.delete(PARAMETER.room);
      } else {
        next.set(PARAMETER.room, room);
      }
      const text = next.toString();
      return text ? `?${text}` : ROUTES.items;
    },
    [parameterText],
  );

  const activeFilterCount = FILTER_PARAMETERS.filter((name) =>
    parameters.has(name),
  ).length;
  return {
    query,
    search: parameters.get(PARAMETER.search) ?? "",
    selectedRoom: parameters.get(PARAMETER.room) ?? "",
    filterValue: (name: ParameterName) => parameters.get(name) ?? "",
    activeFilterCount,
    update,
    clearFilters,
    roomLink,
    flagValue: FLAG_ON,
  };
}
