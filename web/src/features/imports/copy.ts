export const IMPORT_COPY = {
  gettingDetails: (host: string) => `Getting details from ${host}…`,
  gettingDetailsUnknownHost: "Getting details…",
  cancel: "Cancel",
  addManuallyInstead: "Add manually instead",
  reviewTitle: "Review item",
  variantLegend: "Which one did you buy?",
  listingPrice: "Listing price",
  listingPriceHint: "The shop's current price, not what you paid",
  listingSource: (host: string, date: string) =>
    `From ${host} · fetched ${date}`,
  partial: "Some details weren't available.",
  pricePaidPlaceholder: "e.g. 79",
  photosLabel: "Photos from the shop",
  noPhoto: "No photo found",
  keepPhoto: "Keep this photo",
  previousPhoto: "Previous photo",
  nextPhoto: "Next photo",
  photoPosition: (position: number, total: number) =>
    `Photo ${position} of ${total}`,
  photoAlt: (name: string) => `Photo of ${name}`,
  addManually: "Add manually",
  uploadReceipt: "Upload receipt",
  tryAgain: "Try again",
  checkAgain: "Check again",
} as const;

interface ProblemText {
  title: string;
  reason: string;
}

export const GENERIC_PROBLEM: ProblemText = {
  title: "We couldn't get the details",
  reason:
    "Something stopped Keeps reading that page. Add the item yourself — your link is kept.",
};

/** Panel 06 wording per failed-import code. */
export const PROBLEM_COPY: Record<string, ProblemText> = {
  needs_login: {
    title: "This site needs a login",
    reason:
      "We couldn't get the product details because this site needs a login.",
  },
  blocked: {
    title: "This site didn't let Keeps read the page",
    reason:
      "The shop refused the request. Add the item yourself — your link is kept.",
  },
  timeout: {
    title: "The site took too long to answer",
    reason:
      "The shop didn't reply in time. Try again, or add the item yourself.",
  },
  not_found: {
    title: "That page wasn't found",
    reason:
      "The shop says the page doesn't exist. Check the link, or add the item yourself.",
  },
  no_details: {
    title: "No product details found on that page",
    reason: "The page loaded but had no product details Keeps could read.",
  },
  too_large: {
    title: "That page is too large to read",
    reason:
      "The page was bigger than Keeps reads. Add the item yourself — your link is kept.",
  },
  private_document_link: {
    title: "This looks like a private invoice link",
    reason: "Keeps can't open private links. Upload the file instead.",
  },
  invalid_url: {
    title: "That doesn't look like a web link",
    reason: "Check the link, or add the item yourself.",
  },
  not_public: {
    title: "Keeps can only read public shop pages",
    reason:
      "That link points somewhere Keeps can't reach. Add the item yourself.",
  },
  still_waiting: {
    title: "Still getting the details",
    reason:
      "Keeps hasn't finished with this page yet, or is still working through earlier ones. Check again, or add the item yourself — your link is kept.",
  },
};

export const RETRYABLE_CODES = ["timeout", "fetch_failed"];
/** The job may still be running: this page lost touch with it, or stopped waiting. */
export const STILL_WAITING_CODE = "still_waiting";

export const REVIEW_PRICE_PLACEHOLDER = "e.g. 79";
