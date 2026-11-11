// Shapes returned by the Keeps API (/api/v1). Mirrors the Go types.

/** "" means the condition wasn't recorded. */
export type ItemCondition =
  "" | "new" | "like_new" | "good" | "fair" | "poor" | "needs_repair";

export interface Money {
  amount: string;
  currency: string;
}

export type DatePrecision = "date" | "datetime";

export interface PurchaseDate {
  value: string;
  precision: DatePrecision;
}

export interface Reference {
  id: number;
  name: string;
}

export interface Attachment {
  id: number;
  kind: AttachmentKind;
  filename: string;
  mediaType: string;
  sizeBytes: number;
  createdAt: string;
  hasPreview: boolean;
  shared?: boolean;
}

export type AttachmentKind = "photo" | "invoice" | "document";

export interface Listing {
  title: string;
  price: Money | null;
  sourceUrl: string;
  sourceHost: string;
  retrievedAt: string;
}

export interface Item {
  id: number;
  name: string;
  description: string;
  quantity: number;
  brand: string;
  model: string;
  serialNumber: string;
  condition: ItemCondition;
  warrantyUntil: string;
  measurements: Measurements | null;
  parts: Part[];
  units: Unit[];
  notes: string;
  seller: string;
  orderReference: string;
  pricePaid: Money | null;
  purchaseDate: PurchaseDate | null;
  purchaseId: number;
  sharedWith: Reference[];
  room: Reference | null;
  category: Reference | null;
  tags: Reference[];
  collections: Reference[];
  primaryPhotoId: number;
  photos: Attachment[];
  documents: Attachment[];
  listing: Listing | null;
  sourceUrl: string;
  fieldSources: Record<string, "user" | "import">;
  revision: number;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
  possibleDuplicates?: Reference[];
}

export interface ItemSummary {
  id: number;
  name: string;
  pricePaid: Money | null;
  purchaseDate: PurchaseDate | null;
  room: Reference | null;
  primaryPhotoId: number;
  hasReceipt: boolean;
  createdAt: string;
  deletedAt?: string;
}

export interface ItemPage {
  items: ItemSummary[];
  total: number;
  nextCursor?: string;
}

/** Every user-editable field; PUT replaces all of them. */
/** Reusable starting details for similar belongings; no purchase facts. */
export interface Preset {
  id: number;
  name: string;
  room: string;
  category: string;
  tags: string[];
  collections: string[];
  condition: ItemCondition;
  notes: string;
  /** Group names that don't exist; saving an item with them creates them. */
  missing: string[];
}

export type PresetInput = Omit<Preset, "id" | "missing">;

/** A named page of a PDF the owner wants to find again. */
export interface Bookmark {
  id: number;
  page: number;
  label: string;
}

/** A consumable or replacement part the owner recorded as fitting an item. */
export interface Part {
  id: number;
  name: string;
  partNumber: string;
  note: string;
  sourceUrl: string;
}

export type PartInput = Omit<Part, "id">;

/** One of several matching belongings saved as one item, told apart by its label or printed serial. */
export interface Unit {
  id: number;
  label: string;
  serialNumber: string;
}

export type UnitInput = Omit<Unit, "id">;

/** Optional sizes and weight, kept exactly as typed, with their units. */
export interface Measurements {
  width: string;
  height: string;
  depth: string;
  lengthUnit: string;
  weight: string;
  weightUnit: string;
  fitNote: string;
}

export interface ItemInput {
  name: string;
  description: string;
  quantity: number;
  brand: string;
  model: string;
  serialNumber: string;
  condition: ItemCondition;
  warrantyUntil: string;
  measurements: Measurements | null;
  notes: string;
  seller: string;
  orderReference: string;
  pricePaid: Money | null;
  purchaseDate: PurchaseDate | null;
  room: string;
  category: string;
  tags: string[];
  collections: string[];
  purchaseId: number;
  attachmentIds: number[];
}

export interface Revision {
  source: "user" | "import" | "restore";
  changes: Record<string, { from: string; to: string } | boolean>;
  changedAt: string;
}

export type GroupKind = "rooms" | "categories" | "tags" | "collections";

export interface Group {
  id: number;
  name: string;
  itemCount: number;
  thumbnailIds: number[];
  /** Rooms only. */
  description?: string;
  /** Rooms only; absent when the room has no photo. */
  photoId?: number;
}

/** Optional room extras; photoId 0 removes the photo. */
export interface RoomDetails {
  description?: string;
  photoId?: number;
}

/** A referenced photo or document that a backup does not contain. */
export interface MissingFile {
  filename: string;
  copy: "original" | "preview" | "thumbnail";
  items: string[];
  /** "damaged": stored bytes differ from the upload; "" (older backups) means missing. */
  problem: "missing" | "damaged" | "";
}

/** A backup or export the server finished writing, ready to download once. */
export interface PreparedFile {
  downloadUrl: string;
  filename: string;
  complete: boolean;
  missingFiles: MissingFile[];
}

export interface RestoreResult {
  restored: boolean;
  createdAt: string;
  files: number;
  missingFiles: MissingFile[];
}

export interface DocumentEntry extends Attachment {
  itemId: number;
  itemName: string;
}

/** One page of GET /documents?paged=1, newest first. */
export interface DocumentPage {
  documents: DocumentEntry[];
  total: number;
  nextCursor?: string;
}

export interface PasskeySummary {
  id: number;
  name: string;
  createdAt: string;
  lastUsedAt?: string;
}

export type Theme = "dark" | "light";

export interface Account {
  username: string;
  displayName: string;
  theme: Theme;
  householdName: string;
  currency: string;
  totpEnabled: boolean;
  recoveryCodesRemaining: number;
  passkeysAvailable: boolean;
  passkeys: PasskeySummary[];
  csrfToken: string;
  recentlyConfirmed: boolean;
}

export interface BootstrapState {
  claimed: boolean;
  passkeysAvailable: boolean;
}

export interface SessionSummary {
  id: number;
  userAgent: string;
  createdAt: string;
  lastSeenAt: string;
  current: boolean;
}

export interface TotpSetup {
  secret: string;
  uri: string;
  qrCode: string;
  account: string;
}

export type ImportState =
  "queued" | "running" | "done" | "failed" | "cancelled" | "saved";

export interface ImportCandidate {
  name: string;
  description: string;
  brand: string;
  model: string;
  sku: string;
  listingPrice: Money | null;
  availability: string;
  seller: string;
  imageIds: number[];
}

export interface ImportDraft {
  id: string;
  state: ImportState;
  errorCode?: string;
  sourceUrl: string;
  sourceHost: string;
  retrievedAt?: string;
  candidates: ImportCandidate[];
  pageTitle: string;
  partial: boolean;
}

export interface CeremonyStart {
  challengeId: string;
  options: { publicKey: Record<string, unknown> };
}

/** Product details the "Add to Keeps" button reads from the open page. */
export interface PageSnapshot {
  url: string;
  title: string;
  metaTags: { key: string; value: string }[];
  structuredData: string[];
}

/** What was paid for a set of items, per currency (never summed across currencies). */
export interface CostSummary {
  totals: Record<string, string>;
  pricedItems: number;
  unpricedItems: number;
}

export interface RoomCosts {
  rooms: Record<string, CostSummary>;
  allRooms: CostSummary;
  unassigned: CostSummary;
}
