export const ACCOUNT_COPY = {
  pageTitle: "Account",
  tabsLabel: "Account sections",
  tabs: {
    account: "Account",
    security: "Security",
    "sign-in": "Sign-in",
    appearance: "Appearance",
    data: "Data",
  },
  yourAccount: "Your account",
  yourAccountHint: "Change your name and password.",
  name: "Name",
  password: "Password",
  passwordMask: "•••••••",
  change: "Change",
  household: "Household",
  householdHint: "The name and currency Keeps uses for your home.",
  householdName: "Household name",
  currency: "Default currency",
  currencyHint: "A three-letter code such as GBP.",
  notSet: "Not set",
  editLabel: (label: string) => `Edit ${label.toLowerCase()}`,
  saved: (label: string) => `${label} saved.`,
  securityOverviewHint: "Optional ways to keep your account safe.",
} as const;

export const PASSWORD_COPY = {
  title: "Change password",
  current: "Current password",
  newPassword: "New password",
  newPasswordHint: "At least 10 characters. A few words together work well.",
  submit: "Change password",
  submitting: "Changing…",
  changed: "Password changed",
  changedDetail: "Your other sessions were signed out.",
} as const;

export const SECOND_FACTOR_COPY = {
  code: "Authenticator code",
  recoveryCode: "Recovery code",
  useRecoveryCode: "Use a recovery code instead",
  useAuthenticator: "Use authenticator code instead",
} as const;

export const REAUTH_COPY = {
  title: "Confirm it's you",
  hint: "Enter your password to continue. You won't be asked again for a few minutes.",
  password: "Password",
  submit: "Confirm",
  submitting: "Checking…",
} as const;

export const APPEARANCE_COPY = {
  title: "Appearance",
  hint: "Choose how Keeps looks for you.",
  legend: "Theme",
  dark: "Dark",
  recommended: "Recommended",
  light: "Light",
  saveFailed:
    "Couldn't save your theme. It will apply on this device until you reload.",
} as const;

export const PASSKEY_COPY = {
  title: "Passkeys",
  description: "Sign in without a password.",
  unavailable:
    "Passkeys need Keeps to be opened over HTTPS at its configured address.",
  setUp: "Set up",
  nameTitle: "Name this passkey",
  nameLabel: "Passkey name",
  nameHint: "So you can tell your passkeys apart later.",
  defaultName: "This device",
  continue: "Continue",
  creating: "Waiting for your device…",
  renameTitle: "Rename passkey",
  removeTitle: (name: string) => `Remove “${name}”?`,
  removeText:
    "You won't be able to sign in with this passkey any more. Your password still works.",
  remove: "Remove passkey",
  added: (date: string) => `Added ${date}`,
  lastUsed: (relative: string) => `last used ${relative}`,
  neverUsed: "not used yet",
  menuLabel: (name: string) => `More actions for ${name}`,
  none: "No passkeys yet.",
  cancelled: "The passkey wasn't created. Try again when your device asks.",
  saved: "Passkey added",
} as const;

export const AUTHENTICATOR_COPY = {
  title: "Authenticator",
  description: "Use a 6-digit code for an extra layer of security.",
  on: "On",
  setUp: "Set up",
  turnOff: "Turn off",
  turnOffTitle: "Turn off the authenticator?",
  turnOffText:
    "You'll sign in with just your password (or a passkey). Your recovery codes stop working too.",
  turnedOff: "Authenticator turned off",
  setupTitle: "Set up an authenticator",
  scan: "Scan this code with your authenticator app, or type the key below.",
  qrAlt: "QR code for your authenticator app",
  manualKey: "Key",
  copyKey: "Copy key",
  copied: "Copied",
  code: "6-digit code from the app",
  verify: "Verify",
  verifying: "Checking…",
} as const;

export const RECOVERY_COPY = {
  title: "Recovery codes",
  description: "Get one-time codes to regain access if needed.",
  needsAuthenticator: "Available once an authenticator is set up.",
  remaining: (count: number, total: number) => `${count} of ${total} left`,
  newCodes: "New codes",
  newCodesTitle: "Get new recovery codes?",
  newCodesText:
    "Your current codes stop working as soon as the new ones are made.",
  makeCodes: "Make new codes",
  dialogTitle: "Save your recovery codes",
  dialogText:
    "Each code works once if you lose your authenticator. They won't be shown again.",
  listLabel: "Recovery codes",
  download: "Download",
  copy: "Copy",
  copied: "Copied",
  savedCheckbox: "I've saved these codes",
  done: "Done",
  fileHeading: "Keeps recovery codes. Each code works once.",
} as const;

export const SESSIONS_COPY = {
  title: "Where you're signed in",
  hint: "Sign out anywhere you don't recognise.",
  thisDevice: "This device",
  lastActive: (relative: string) => `Active ${relative}`,
  signOut: "Sign out",
  signOutLabel: (device: string) => `Sign out ${device}`,
  signOutOthers: "Sign out all other sessions",
  signedOut: "Signed out",
  othersSignedOut: "Other sessions signed out",
  onlyThisDevice: "You're only signed in on this device.",
  unknownDevice: "Unknown device",
  deviceOn: (browser: string, system: string) => `${browser} on ${system}`,
} as const;

export const TIME_COPY = {
  justNow: "just now",
} as const;
