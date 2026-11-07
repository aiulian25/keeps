export const ACCOUNT_TABS = [
  "account",
  "security",
  "sign-in",
  "appearance",
  "data",
] as const;
export type AccountTab = (typeof ACCOUNT_TABS)[number];
export const DEFAULT_ACCOUNT_TAB: AccountTab = "account";

export const REAUTHENTICATION_REQUIRED_CODE = "reauthentication_required";
export const TOTAL_RECOVERY_CODES = 10;
export const MANUAL_KEY_GROUP_SIZE = 4;
export const AUTHENTICATOR_CODE_LENGTH = 6;
export const CURRENCY_CODE_LENGTH = 3;
export const RECOVERY_CODES_FILENAME = "keeps-recovery-codes.txt";
export const PLAIN_TEXT_TYPE = "text/plain";
export const COPIED_NOTICE_MS = 2000;

/** Checked in order; the first match names the browser or system. */
export const BROWSER_PATTERNS = [
  { pattern: /Edg\//, name: "Edge" },
  { pattern: /OPR\//, name: "Opera" },
  { pattern: /Firefox\//, name: "Firefox" },
  { pattern: /Chrome\//, name: "Chrome" },
  { pattern: /Safari\//, name: "Safari" },
];

export const SYSTEM_PATTERNS = [
  { pattern: /iPhone/, name: "iPhone" },
  { pattern: /iPad/, name: "iPad" },
  { pattern: /Android/, name: "Android" },
  { pattern: /CrOS/, name: "ChromeOS" },
  { pattern: /Windows/, name: "Windows" },
  { pattern: /Mac OS X|Macintosh/, name: "macOS" },
  { pattern: /Linux/, name: "Linux" },
];
