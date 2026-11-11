export const SEARCH_DEBOUNCE_MS = 250;
export const SAVED_BADGE_MS = 2500;
export const TOAST_MS = 6000;
export const IMPORT_POLL_MS = 800;
export const PAGE_SIZE = 48;
export const SINGLE_QUANTITY = 1;
const MAXIMUM_UPLOAD_MEGABYTES = 20;
const BYTES_PER_MEGABYTE = 1_048_576;
export const MAXIMUM_UPLOAD_BYTES =
  MAXIMUM_UPLOAD_MEGABYTES * BYTES_PER_MEGABYTE;
export const PASSWORD_MINIMUM_LENGTH = 10;
export const BACKUP_PASSPHRASE_MINIMUM = 8;
export const ACCEPTED_PHOTO_TYPES = "image/jpeg,image/png,image/webp";
export const ACCEPTED_DOCUMENT_TYPES =
  "application/pdf,image/jpeg,image/png,image/webp";
export const BYTES_PER_KILOBYTE = 1024;
const CAPTURE_LIFETIME_MINUTES = 30;
const MILLISECONDS_PER_MINUTE = 60_000;
/** How long Add to Keeps page details are kept for a retry or refresh. */
export const CAPTURE_LIFETIME_MS =
  CAPTURE_LIFETIME_MINUTES * MILLISECONDS_PER_MINUTE;
const CAPTURE_MAXIMUM_KILOCHARACTERS = 512;
export const CAPTURE_MAXIMUM_CHARACTERS =
  CAPTURE_MAXIMUM_KILOCHARACTERS * BYTES_PER_KILOBYTE;
