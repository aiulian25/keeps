import type { ItemInput, PurchaseDate } from "../../api/types";

/**
 * What other screens may hand to the manual form through router state.
 * Home and the import screens pass `url`; item detail passes the purchase
 * fields for "Add another item from this purchase".
 */
export interface ManualItemState {
  url?: string;
  focusInvoice?: boolean;
  purchaseId?: number;
  purchaseItemName?: string;
  seller?: string;
  orderReference?: string;
  purchaseDate?: PurchaseDate | null;
  fields?: Partial<ItemInput>;
}

/** Router state for the Add item page (Home passes it). */
export interface AddItemState {
  url?: string;
  autoStart?: boolean;
  /** The room open on the Items page: a new item starts in it. */
  room?: string;
}
