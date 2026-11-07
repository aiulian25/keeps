export const TRASH_COPY = {
  title: "Recently deleted",
  backLabel: "Back to items",
  deletedOn: (date: string) => `Deleted ${date}`,
  restore: "Restore",
  restoreLabel: (name: string) => `Restore ${name}`,
  deletePermanently: "Delete permanently",
  deletePermanentlyLabel: (name: string) => `Delete permanently: ${name}`,
  confirmTitle: (name: string) => `Delete “${name}” permanently?`,
  confirmText:
    "Its photos and documents are removed too unless another item uses them. This can't be undone.",
  restored: (name: string) => `Restored “${name}”`,
  view: "View",
  purged: (name: string) => `Deleted “${name}” permanently`,
  empty: "Nothing here. Items you delete appear here so you can restore them.",
  showMore: "Show more",
} as const;
