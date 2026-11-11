export const UPLOAD_COPY = {
  linking: "Adding to the item…",
  linkFailed:
    "Uploaded, but not added to this item yet. Retry adds the same file without uploading it again.",
  refreshFailed:
    "The file was added, but the list couldn't be refreshed. Reload the page to see it.",
  addPhoto: "Add a photo",
  addYourPhotos: "Add your own photos",
  addInvoice: "Add an invoice",
  addDocument: "Add a manual or other document",
  dragHint: "Drag and drop, or click to upload",
  photoFormats: "JPG, PNG or WebP",
  documentFormats: "PDF, JPG, PNG or WebP",
  uploadPhotos: "Upload photos",
  uploadReceipt: "Upload receipt or invoice",
  receiptHint: "Counts as proof of purchase. PDF, JPG, PNG or WebP",
  uploadOtherDocument: "Upload manual or other document",
  uploadInvoice: "Upload invoice",
  uploading: "Uploading",
  failed: "Upload failed",
  cancelUpload: (name: string) => `Cancel upload of ${name}`,
  removeFile: (name: string) => `Remove ${name}`,
  retryUpload: (name: string) => `Retry ${name}`,
  tooLarge: "Files can be up to 20 MB.",
} as const;

export const MAIN_PHOTO_COPY = {
  mainPhoto: "Main photo",
  useAsMain: "Use as main photo",
  makeMain: (name: string) => `Use ${name} as the main photo`,
  isMain: (name: string) => `${name} is the main photo`,
} as const;
