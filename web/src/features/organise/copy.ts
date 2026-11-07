import type { GroupKind } from "../../api/types";

interface KindCopy {
  title: string;
  singular: string;
  create: string;
  empty: string;
}

export const KIND_COPY: Record<GroupKind, KindCopy> = {
  rooms: {
    title: "Rooms",
    singular: "room",
    create: "Create room",
    empty: "No rooms yet. Create one, or add a room while editing an item.",
  },
  categories: {
    title: "Categories",
    singular: "category",
    create: "Create category",
    empty:
      "No categories yet. Create one, or add a category while editing an item.",
  },
  tags: {
    title: "Tags",
    singular: "tag",
    create: "Create tag",
    empty: "No tags yet. Create one, or add a tag while editing an item.",
  },
  collections: {
    title: "Collections",
    singular: "collection",
    create: "Create collection",
    empty:
      "No collections yet. Create one, or add a collection while editing an item.",
  },
};

export const ORGANISE_COPY = {
  tabsLabel: "Organise by",
  name: "Name",
  create: "Create",
  itemCount: (count: number) => (count === 1 ? "1 item" : `${count} items`),
  moreThumbnails: (count: number) => `+${count}`,
  menuLabel: (name: string) => `More actions for ${name}`,
  renameTitle: (singular: string) => `Rename ${singular}`,
  deleteTitle: (name: string) => `Delete ${name}?`,
  deleteText: (itemCount: string) => `Its ${itemCount} stay in Keeps.`,
  deleteEmptyText: "It has no items.",
  moveTo: "Move its items to:",
  leaveUnassigned: "Leave them unassigned",
  created: (name: string) => `Created ${name}`,
  renamed: (name: string) => `Renamed to ${name}`,
  deleted: (name: string) => `Deleted ${name}`,
} as const;

export const ROOM_COPY = {
  createTitle: "Create room",
  editTitle: (name: string) => `Edit ${name}`,
  edit: "Edit",
  editRoom: "Edit room",
  total: (amount: string) => `${amount} total`,
  name: "Name",
  description: "Description",
  descriptionHint: "For example where it is, or what's kept there.",
  photo: "Room photo",
  addPhoto: "Add a room photo",
  changePhoto: "Change photo",
  removePhoto: "Remove photo",
  waitingForUpload: "Waiting for the photo to upload…",
  saved: (name: string) => `Saved ${name}`,
} as const;

export const VIEW_COPY = {
  label: "View",
  cards: "Cards",
  list: "List",
  wide: "Wide",
} as const;

export const COST_COPY = {
  allRooms: "All rooms",
  notInRoom: "Not in a room",
  noPrices: "No prices yet",
  withoutPrice: (count: number) =>
    count === 1 ? "1 item without a price" : `${count} items without a price`,
  summaryLabel: "Cost of items in rooms",
} as const;
