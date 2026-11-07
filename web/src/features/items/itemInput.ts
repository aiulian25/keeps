import type { Item, ItemInput } from "../../api/types";

export function emptyItemInput(): ItemInput {
  return {
    name: "",
    description: "",
    quantity: 1,
    brand: "",
    model: "",
    serialNumber: "",
    condition: "",
    warrantyUntil: "",
    measurements: null,
    notes: "",
    seller: "",
    orderReference: "",
    pricePaid: null,
    purchaseDate: null,
    room: "",
    category: "",
    tags: [],
    collections: [],
    purchaseId: 0,
    attachmentIds: [],
  };
}

/** The editable form of a saved item; attachments are managed separately. */
export function inputFromItem(item: Item): ItemInput {
  return {
    name: item.name,
    description: item.description,
    quantity: item.quantity,
    brand: item.brand,
    model: item.model,
    serialNumber: item.serialNumber,
    condition: item.condition,
    warrantyUntil: item.warrantyUntil,
    measurements: item.measurements,
    notes: item.notes,
    seller: item.seller,
    orderReference: item.orderReference,
    pricePaid: item.pricePaid,
    purchaseDate: item.purchaseDate,
    room: item.room?.name ?? "",
    category: item.category?.name ?? "",
    tags: item.tags.map((tag) => tag.name),
    collections: item.collections.map((collection) => collection.name),
    purchaseId: item.purchaseId,
    attachmentIds: [],
  };
}

export function sameInput(first: ItemInput, second: ItemInput) {
  return JSON.stringify(first) === JSON.stringify(second);
}
