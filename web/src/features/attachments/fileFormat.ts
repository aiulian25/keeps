import type { Attachment } from "../../api/types";
import { BYTES_PER_KILOBYTE } from "../../constants/limits";

export const PDF_MEDIA_TYPE = "application/pdf";
const KILOBYTE_UNIT = "KB";
const MEGABYTE_UNIT = "MB";
const SEPARATOR = " · ";
const TYPE_LABELS: Record<string, string> = {
  [PDF_MEDIA_TYPE]: "PDF",
  "image/jpeg": "JPG",
  "image/png": "PNG",
  "image/webp": "WebP",
};

const fileDateFormat = new Intl.DateTimeFormat(undefined, {
  day: "numeric",
  month: "short",
  year: "numeric",
});

function fileTypeLabel(mediaType: string) {
  return (
    TYPE_LABELS[mediaType] ?? mediaType.split("/").pop()?.toUpperCase() ?? ""
  );
}

function formatSize(bytes: number) {
  const kilobytes = bytes / BYTES_PER_KILOBYTE;
  if (kilobytes < BYTES_PER_KILOBYTE) {
    return `${Math.max(1, Math.round(kilobytes))} ${KILOBYTE_UNIT}`;
  }
  return `${(kilobytes / BYTES_PER_KILOBYTE).toFixed(1)} ${MEGABYTE_UNIT}`;
}

function formatFileDate(timestamp: string) {
  const date = new Date(timestamp);
  return Number.isNaN(date.getTime()) ? "" : fileDateFormat.format(date);
}

/** "PDF · 412 KB · 12 Mar 2024" */
export function fileSummary(attachment: Attachment) {
  return [
    fileTypeLabel(attachment.mediaType),
    formatSize(attachment.sizeBytes),
    formatFileDate(attachment.createdAt),
  ]
    .filter(Boolean)
    .join(SEPARATOR);
}
