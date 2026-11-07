import {
  Bath,
  Bed,
  Car,
  House,
  Layers,
  Monitor,
  Sofa,
  Tag,
  Tags,
  Utensils,
  type LucideIcon,
} from "lucide-react";
import type { GroupKind } from "../../api/types";

export const GROUP_KINDS: GroupKind[] = [
  "rooms",
  "categories",
  "tags",
  "collections",
];

export const KIND_ICONS: Record<GroupKind, LucideIcon> = {
  rooms: Sofa,
  categories: Tag,
  tags: Tags,
  collections: Layers,
};

/** The Items page filter parameter for each kind (`/items?room=3`). */
export const ITEM_FILTER_PARAMETERS: Record<GroupKind, string> = {
  rooms: "room",
  categories: "category",
  tags: "tag",
  collections: "collection",
};

/** An item has one room and one category, so only those can be moved on delete. */
export const KINDS_WITH_MOVE: GroupKind[] = ["rooms", "categories"];

export const MAXIMUM_THUMBNAILS = 4;

/** Matched against the lower-cased room name, first match wins. */
export const ROOM_ICONS: { match: string; icon: LucideIcon }[] = [
  { match: "kitchen", icon: Utensils },
  { match: "bathroom", icon: Bath },
  { match: "bedroom", icon: Bed },
  { match: "living", icon: Sofa },
  { match: "lounge", icon: Sofa },
  { match: "office", icon: Monitor },
  { match: "study", icon: Monitor },
  { match: "garage", icon: Car },
];

export const DEFAULT_ROOM_ICON = House;
