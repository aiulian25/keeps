export const DATA_COPY = {
  title: "Data and recovery",
  hint: "Keep your data safe and in your control.",
  export: "Export",
  exportDescription:
    "Download your data as a file (e.g. JSON) to keep or move elsewhere.",
  backup: "Backup",
  backupDescription: "Create a backup of your Keeps data.",
  backUpNow: "Back up now",
  restore: "Restore",
  restoreDescription: "Restore from a previous backup file.",
  recentlyDeleted: "Recently deleted",
  recentlyDeletedDescription: "View and restore items you've deleted.",
  view: "View",
  downloadStarting: "Your download will start in a moment.",
} as const;

export const ABOUT_COPY = {
  title: "About Keeps",
  hint: "Which release this Keeps is running, for upgrades and support.",
  version: "Version",
  developmentBuild: "Development build",
} as const;

export const MISSING_COPY = {
  label: "Missing or damaged photos and documents",
  damaged:
    "— damaged: it no longer matches what was uploaded. Use an older backup or upload it again.",
  copyName: {
    original: "the file itself",
    preview: "its preview",
    thumbnail: "its small preview",
  },
  notLinked: "Not linked to an item",
  line: (filename: string, copy: string, items: string) =>
    `${filename} (${copy}) — ${items}`,
} as const;

export const EXPORT_COPY = {
  title: "Export your data",
  legend: "Format",
  csv: "Spreadsheet (CSV)",
  csvDetail:
    "Item details in one table for Excel or Numbers. No photos or documents.",
  archive: "Full archive (files included)",
  archiveDetail:
    "Every item and room, with their photos and documents, as a .zip you can move elsewhere. Recently deleted items, history and sign-in details aren't included: use a backup to move a whole Keeps.",
  download: "Download",
  preparing: "Preparing…",
} as const;

export const BACKUP_COPY = {
  title: "Back up Keeps",
  intro:
    "The backup is encrypted with a passphrase you choose. It includes everything needed to restore Keeps on a new machine.",
  warning:
    "Keep the passphrase somewhere safe. The backup can't be opened without it, and nobody can recover it for you.",
  passphrase: "Passphrase",
  passphraseHint: (minimum: number) => `At least ${minimum} characters.`,
  confirm: "Type the passphrase again",
  tooShort: (minimum: number) => `Use at least ${minimum} characters.`,
  mismatch: "The passphrases don't match.",
  submit: "Download backup",
  preparing: "Preparing…",
  incompleteTitle: "This backup is incomplete",
  incompleteText: (count: number) =>
    count === 1
      ? "1 file Keeps refers to is missing from storage or no longer matches what was uploaded, so the backup can't hold a good copy of it. Everything else is fine. You can still download it."
      : `${count} files Keeps refers to are missing from storage or no longer match what was uploaded, so the backup can't hold good copies of them. Everything else is fine. You can still download it.`,
  downloadAnyway: "Download anyway",
  startedTitle: "Backup started",
  startedDetail: (filename: string) =>
    `Your browser is downloading ${filename}. Store it somewhere off this machine.`,
} as const;

export const RESTORE_COPY = {
  title: "Restore from a backup",
  intro:
    "Restoring replaces everything in Keeps with the contents of the backup, and signs everyone out. Your current data is kept as a checkpoint until the restore succeeds.",
  file: "Backup file",
  fileHint: "A .keeps.age file made by Back up now.",
  chooseFile: "Choose a backup file.",
  selectedFile: (name: string) => `Selected: ${name}`,
  passphrase: "Backup passphrase",
  continue: "Continue",
  confirmTitle: "Replace all your data?",
  confirmText: (filename: string) =>
    `Everything in Keeps will be replaced with ${filename}. Everyone, including you, will be signed out.`,
  back: "Back",
  replace: "Replace my data",
  uploading: (percent: number) => `Uploading… ${percent}%`,
  checking: "Checking and restoring the backup…",
  stayHere: "Keep this page open until it finishes.",
  stagesLabel: "Restore progress",
  stageUpload: "Uploading",
  stageCheck: "Checking",
  stageRestore: "Restoring",
  incompleteTitle: "This backup has missing or damaged files",
  incompleteText: (count: number) =>
    count === 1
      ? "1 photo or document in this backup is missing or damaged. Restoring brings back everything else, but that file will be gone or wrong. Nothing has been changed yet."
      : `${count} photos or documents in this backup are missing or damaged. Restoring brings back everything else, but those files will be gone or wrong. Nothing has been changed yet.`,
  restoreAnyway: "Restore without them",
  doneMissing: (count: number) =>
    count === 1
      ? "1 file from the list was not in the backup."
      : `${count} files from the list were not in the backup.`,
  doneTitle: "Restore complete",
  doneText: (date: string, files: number) =>
    `Keeps now holds the backup from ${date} (${files} files). Everyone has been signed out.`,
  signInAgain: "Sign in again",
} as const;
