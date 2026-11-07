import { useRef, useState, type DragEvent, type ReactNode } from "react";
import { RotateCcw, Star, X } from "lucide-react";
import { attachmentUrl } from "../../api/client";
import type { AttachmentKind } from "../../api/types";
import {
  ACCEPTED_DOCUMENT_TYPES,
  ACCEPTED_PHOTO_TYPES,
} from "../../constants/limits";
import { MAIN_PHOTO_COPY, UPLOAD_COPY } from "./copy";
import type { Upload } from "./useUploads";
import styles from "./DropZone.module.css";

interface DropZoneProps {
  kind: AttachmentKind;
  title: string;
  formats: string;
  icon: ReactNode;
  uploads: Upload[];
  onFiles: (files: FileList, kind: AttachmentKind) => void;
  onRetry: (upload: Upload) => void;
  onRemove: (key: string) => void;
  variant?: "dashed" | "tile";
  multiple?: boolean;
  mainPhotoId?: number | null;
  onMakeMain?: (attachmentId: number) => void;
}

function needsRetry(upload: Upload) {
  return upload.status === "failed" || upload.status === "linkFailed";
}

/** Click or drop to upload; the native file input keeps it keyboard-operable. */
export function DropZone({
  kind,
  title,
  formats,
  icon,
  uploads,
  onFiles,
  onRetry,
  onRemove,
  variant = "dashed",
  multiple = true,
  mainPhotoId = null,
  onMakeMain,
}: DropZoneProps) {
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const accept =
    kind === "photo" ? ACCEPTED_PHOTO_TYPES : ACCEPTED_DOCUMENT_TYPES;
  const onDrop = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setDragging(false);
    if (event.dataTransfer.files.length > 0) {
      onFiles(event.dataTransfer.files, kind);
    }
  };
  const mine = uploads.filter((upload) => upload.kind === kind);
  const finishedPhotos = mine.filter((upload) => upload.attachment);
  const canChooseMain = onMakeMain !== undefined && finishedPhotos.length > 1;
  const chooseMain = (attachmentId: number) => () => onMakeMain?.(attachmentId);
  return (
    <div className={styles.wrap}>
      <label
        className={styles.zone}
        data-variant={variant}
        data-dragging={dragging || undefined}
        onDragOver={(event) => {
          event.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
      >
        <input
          ref={input}
          type="file"
          className="visually-hidden"
          accept={accept}
          multiple={multiple}
          onChange={(event) => {
            if (event.target.files && event.target.files.length > 0) {
              onFiles(event.target.files, kind);
            }
            event.target.value = "";
          }}
        />
        <span className={styles.icon}>{icon}</span>
        <span className={styles.title}>{title}</span>
        {variant === "dashed" && (
          <span className={styles.hint}>{UPLOAD_COPY.dragHint}</span>
        )}
        <span className={styles.hint}>{formats}</span>
      </label>
      {mine.length > 0 && (
        <ul className={styles.list}>
          {mine.map((upload) => (
            <li key={upload.key} data-status={upload.status}>
              {upload.attachment?.hasPreview && (
                <img
                  className={styles.uploadThumbnail}
                  src={attachmentUrl(upload.attachment.id, "thumbnail")}
                  alt=""
                />
              )}
              <span className={styles.name}>{upload.file.name}</span>
              {upload.status === "uploading" && (
                <progress
                  max={1}
                  value={upload.progress}
                  aria-label={`${UPLOAD_COPY.uploading} ${upload.file.name}`}
                />
              )}
              {upload.status === "linking" && (
                <span className={styles.hint} role="status">
                  {UPLOAD_COPY.linking}
                </span>
              )}
              {needsRetry(upload) && (
                <span className={styles.error}>{upload.error}</span>
              )}
              {canChooseMain && upload.attachment && (
                <button
                  type="button"
                  className={styles.mainChoice}
                  aria-pressed={upload.attachment.id === mainPhotoId}
                  aria-label={
                    upload.attachment.id === mainPhotoId
                      ? MAIN_PHOTO_COPY.isMain(upload.file.name)
                      : MAIN_PHOTO_COPY.makeMain(upload.file.name)
                  }
                  onClick={chooseMain(upload.attachment.id)}
                >
                  <Star aria-hidden="true" />
                </button>
              )}
              {needsRetry(upload) && (
                <button
                  type="button"
                  aria-label={UPLOAD_COPY.retryUpload(upload.file.name)}
                  onClick={() => onRetry(upload)}
                >
                  <RotateCcw aria-hidden="true" />
                </button>
              )}
              <button
                type="button"
                aria-label={
                  upload.status === "uploading"
                    ? UPLOAD_COPY.cancelUpload(upload.file.name)
                    : UPLOAD_COPY.removeFile(upload.file.name)
                }
                onClick={() => onRemove(upload.key)}
              >
                <X aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
