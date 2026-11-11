export const ADD_COPY = {
  title: "Add item",
  subtitle: "Add a purchase or belongings to your home.",
  urlLabel: "Product link",
  urlPlaceholder: "Paste product link (e.g. from Amazon, Currys, Argos)",
  getDetails: "Get details",
  starting: "Getting details…",
  addManually: "Add manually",
  uploadReceipt: "Upload receipt",
} as const;

/** Friendly text for the link checks done before an import starts. */
export const LINK_ERRORS: Record<string, string> = {
  invalid_url:
    "Check the link: it should be a shop page address starting with https://",
  not_public:
    "Keeps can only read public shop pages. Paste the product page link instead.",
  private_document_link:
    "This looks like a private invoice link. Upload the file instead.",
};

export const PRIVATE_DOCUMENT_CODE = "private_document_link";

export const MANUAL_COPY = {
  title: "Add item manually",
  nameRequired: "Enter a name for the item.",
  waitingForUploads: "Waiting for uploads…",
  linkKept: (url: string) => `Link: ${url} (kept for reference)`,
  notesLinkPrefix: "Link: ",
  samePurchase: (name: string) => `Same purchase as ${name}.`,
  sharedPurchaseFacts:
    "The seller, order reference, date and receipts are shared: a change here changes them for both items.",
  carriedFiles: (count: number) =>
    count === 1
      ? "1 file from the link will be attached."
      : `${count} files from the link will be attached.`,
  chooseAgain: (count: number) =>
    count === 1
      ? "1 file hadn't finished uploading when the page reloaded. Choose it again."
      : `${count} files hadn't finished uploading when the page reloaded. Choose them again.`,
} as const;

export const MANUAL_DRAFT_STORAGE_KEY = "keeps:manual-item-draft";
/** Drafts saved before this version hold only the form's field values. */
export const MANUAL_DRAFT_VERSION = 1;
export const CAPTURE_STORAGE_KEY = "keeps:add-to-keeps-capture";

export const BUTTON_COPY = {
  title: "Add to Keeps button",
  intro:
    "Some shops block Keeps from reading their pages. This button adds the page you're looking at instead, straight from your browser.",
  stepsLabel: "Set it up once",
  stepShowBar: "Show your bookmarks bar (Ctrl+Shift+B, or ⌘+Shift+B on a Mac).",
  stepDrag: "Drag this button onto the bookmarks bar:",
  buttonLabel: "Add to Keeps",
  stepUse:
    "On a product page, click Add to Keeps. A Keeps tab opens with the details to review.",
  privacy:
    "The button reads only the product details the page publishes for search engines and link previews. They go from your browser to Keeps; Keeps doesn't contact the shop for the page.",
  phoneTitle: "On a phone or tablet",
  phoneText:
    "Copy the button code, add any page as a bookmark, then edit that bookmark and paste the code as its address.",
  copyCode: "Copy button code",
  copied: "Copied",
  dragHint: "Drag me to your bookmarks bar",
  receiving: "Reading the page details…",
  unreadable:
    "Keeps couldn't read what the button sent. Try the button again on the product page, or add the item manually.",
  shopBlocks: "Shop blocks Keeps? Use the Add to Keeps button",
  backToAdd: "Add item",
  retry: "Try again",
  enterManually: "Enter it manually",
  cancel: "Discard these page details",
  expired:
    "These page details were kept for 30 minutes and have expired. Use the button on the product page again.",
} as const;

export const PRESET_COPY = {
  use: "Use a preset",
  title: "Presets",
  choose: "Choose a preset",
  none: "No presets yet. Fill in a room, tags or notes, then save them here to reuse.",
  apply: "Fill in",
  willFill: "This fills in:",
  nothingToFill: "Everything this preset holds is already filled in.",
  keeps: (fields: string) => `Keeps what you typed in: ${fields}.`,
  missing: (names: string) =>
    `Not in Keeps yet: ${names}. Saving the item adds them.`,
  remove: (name: string) => `Delete preset ${name}`,
  saveAs: "Save these details as a preset",
  name: "Name",
  namePlaceholder: "e.g. Garage tools",
  saveHint:
    "Keeps the room, category, tags, collections, condition and notes. Never prices, serial numbers or receipts.",
  save: "Save preset",
} as const;
