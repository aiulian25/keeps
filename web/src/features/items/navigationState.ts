import type { Reference } from "../../api/types";

/** Router state the item detail page understands (set by create flows and item cards). */
export interface ItemDetailState {
  saved?: boolean;
  edit?: boolean;
  possibleDuplicates?: Reference[];
}

export function itemDetailState(state: unknown): ItemDetailState {
  if (typeof state !== "object" || state === null) {
    return {};
  }
  return state as ItemDetailState;
}
