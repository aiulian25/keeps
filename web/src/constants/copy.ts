// Shared user-facing words. Feature-specific copy lives beside each feature.
export const APPLICATION_NAME = "Keeps";

export const NAVIGATION = {
  unavailableTitle: "Can't reach Keeps",
  unavailableText:
    "Keeps didn't answer. Check that this device is online and the server is running, then try again.",
  label: "Main",
  home: "Home",
  items: "Items",
  rooms: "Rooms",
  documents: "Documents",
  account: "Account",
  skipToContent: "Skip to content",
} as const;

export const COMMON = {
  save: "Save",
  saving: "Saving…",
  saved: "Saved",
  cancel: "Cancel",
  close: "Close",
  edit: "Edit",
  delete: "Delete",
  remove: "Remove",
  rename: "Rename",
  back: "Back",
  retry: "Try again",
  more: "More",
  moreDetails: "More details",
  fewerDetails: "Fewer details",
  optional: "(optional)",
  required: "required",
  loading: "Loading…",
  undo: "Undo",
  show: "Show",
  hide: "Hide",
  showPassword: "Show password",
  hidePassword: "Hide password",
  unknown: "—",
  add: "Add",
  or: "or",
  signOut: "Sign out",
  signOutFailed: "You're still signed in",
  signOutFailedDetail:
    "Keeps couldn't end this session, so this device still has access. Check the connection and try again.",
  tryAgain: "Try again",
  continue: "Continue",
  download: "Download",
} as const;

export const ERRORS = {
  offline: "Couldn't reach Keeps. Your changes are still here.",
  sessionExpired:
    "Your session ended. Sign in again to keep going — your changes are still here.",
  generic: "Something went wrong. Please try again.",
  checkFields: "Check the highlighted fields.",
} as const;
