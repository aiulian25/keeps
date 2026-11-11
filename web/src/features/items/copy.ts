import type { ItemCondition } from "../../api/types";

export const ITEM_COPY = {
  name: "Name",
  namePlaceholder: "e.g. KitchenAid Stand Mixer",
  pricePaid: "Price paid",
  pricePlaceholder: "e.g. 349.99",
  purchaseDate: "Purchase date",
  room: "Room",
  roomPlaceholder: "e.g. Kitchen",
  category: "Category",
  categoryPlaceholder: "e.g. Appliances",
  tags: "Tags",
  tagsPlaceholder: "Type and press Enter",
  collections: "Collections",
  collectionsPlaceholder: "e.g. Camping kit",
  seller: "Seller",
  sellerPlaceholder: "e.g. John Lewis",
  orderReference: "Order number",
  brand: "Brand",
  model: "Model",
  serialNumber: "Serial number",
  quantity: "Quantity",
  condition: "Condition",
  conditionNotSet: "Not set",
  warrantyUntil: "Warranty until",
  measurements: "Measurements",
  width: "Width",
  height: "Height",
  depth: "Depth",
  lengthUnit: "Size unit",
  weight: "Weight",
  weightUnit: "Weight unit",
  fitNote: "Fitting note",
  fitNotePlaceholder: "e.g. Needs an 85 cm doorway; wall-mounted",
  fitNoteCount: (length: number, maximum: number) =>
    `${length} of ${maximum} characters`,
  fitNoteTooLong: (length: number, maximum: number) =>
    `The fitting note has ${length} characters; keep it to ${maximum}. Product details belong in Description.`,
  size: "Size",
  warranty: "Warranty",
  inWarranty: "In warranty",
  inWarrantyUntil: (date: string) => `In warranty until ${date}`,
  warrantyEnded: (date: string) => `Warranty ended ${date}`,
  description: "Description",
  notes: "Notes",
  listingPrice: "Listing price",
  saveItem: "Save item",
  saveChanges: "Save changes",
  removeTag: (name: string) => `Remove ${name}`,
} as const;

/** Condition choices in order, best first. */
export const CONDITION_OPTIONS: { value: ItemCondition; label: string }[] = [
  { value: "new", label: "New" },
  { value: "like_new", label: "Like new" },
  { value: "good", label: "Good" },
  { value: "fair", label: "Fair" },
  { value: "poor", label: "Poor" },
  { value: "needs_repair", label: "Needs repair" },
];

export function conditionLabel(condition: ItemCondition) {
  return (
    CONDITION_OPTIONS.find((option) => option.value === condition)?.label ?? ""
  );
}

export const LENGTH_UNITS = ["mm", "cm", "m", "in", "ft"].map((unit) => ({
  value: unit,
  label: unit,
}));

export const WEIGHT_UNITS = ["g", "kg", "lb", "oz"].map((unit) => ({
  value: unit,
  label: unit,
}));

export const PARTS_COPY = {
  title: (count: number) =>
    count === 0 ? "Parts and consumables" : `Parts and consumables (${count})`,
  disclaimer:
    "What fits, as you noted it. Keeps doesn't check compatibility; follow the maker's guidance when replacing parts.",
  add: "Add a part",
  name: "Part",
  namePlaceholder: "e.g. Vacuum bags",
  partNumber: "Part number",
  note: "Compatibility note",
  notePlaceholder: "e.g. Type G only; pack of 5",
  sourceUrl: "Where to find it",
  sourceHint: "A web address kept for reference. Keeps doesn't open it.",
  source: "Source",
  save: "Save part",
  edit: (name: string) => `Edit ${name}`,
  remove: (name: string) => `Remove ${name}`,
} as const;

export const UNITS_COPY = {
  title: (identified: number, quantity: number) =>
    identified === 0
      ? "Individual units"
      : `Individual units (${identified} of ${quantity})`,
  explanation:
    "Tell matching units apart, like a left and a right speaker, by their own serial numbers. The quantity, price and receipts stay with the item.",
  add: "Identify a unit",
  label: "Label",
  labelPlaceholder: "e.g. Left speaker",
  serialNumber: "Serial number",
  save: "Save unit",
  unnamed: (position: number) => `Unit ${position}`,
  allIdentified:
    "Every unit is identified. Raise the quantity to identify another.",
  edit: (name: string) => `Edit ${name}`,
  remove: (name: string) => `Remove ${name}`,
} as const;
