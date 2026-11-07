import { useCallback, useRef, useState } from "react";
import { ApiError } from "../../api/client";
import { attachmentsApi } from "../../api/endpoints";
import type { Attachment, AttachmentKind } from "../../api/types";
import { MAXIMUM_UPLOAD_BYTES } from "../../constants/limits";
import { UPLOAD_COPY } from "./copy";

export interface Upload {
  key: string;
  file: File;
  kind: AttachmentKind;
  progress: number;
  /** "linking": uploaded, being added to the item; "linkFailed": uploaded, not added yet. */
  status: "uploading" | "linking" | "done" | "failed" | "linkFailed";
  error?: string;
  attachment?: Attachment;
}

function failureMessage(error: unknown, fallback: string) {
  return error instanceof ApiError ? error.message : fallback;
}

/**
 * Uploads files as soon as they are chosen, independently of saving the item,
 * so a failed upload never loses the typed details and can be retried alone.
 * When `onUploaded` adds the file to an item, the upload is done only once
 * that has worked too; if it fails, Retry adds the same uploaded file again
 * instead of uploading it a second time.
 */
export function useUploads(onUploaded?: (attachment: Attachment) => unknown) {
  const [uploads, setUploads] = useState<Upload[]>([]);
  const controllers = useRef(new Map<string, AbortController>());

  const update = (key: string, change: Partial<Upload>) =>
    setUploads((current) =>
      current.map((upload) =>
        upload.key === key ? { ...upload, ...change } : upload,
      ),
    );

  /** Adds an uploaded file to its item, if there is one, before calling it done. */
  const finish = useCallback(
    (key: string, attachment: Attachment) => {
      if (!onUploaded) {
        update(key, { status: "done" });
        return Promise.resolve();
      }
      update(key, { status: "linking", error: undefined });
      return Promise.resolve(onUploaded(attachment))
        .then(() => update(key, { status: "done" }))
        .catch((error: unknown) =>
          update(key, {
            status: "linkFailed",
            error: failureMessage(error, UPLOAD_COPY.linkFailed),
          }),
        );
    },
    [onUploaded],
  );

  const start = useCallback(
    (file: File, kind: AttachmentKind, key: string = crypto.randomUUID()) => {
      if (file.size > MAXIMUM_UPLOAD_BYTES) {
        setUploads((current) => [
          ...current.filter((upload) => upload.key !== key),
          {
            key,
            file,
            kind,
            progress: 0,
            status: "failed",
            error: UPLOAD_COPY.tooLarge,
          },
        ]);
        return;
      }
      const controller = new AbortController();
      controllers.current.set(key, controller);
      setUploads((current) => [
        ...current.filter((upload) => upload.key !== key),
        { key, file, kind, progress: 0, status: "uploading" },
      ]);
      attachmentsApi
        .upload(
          kind,
          file,
          (progress) => update(key, { progress }),
          controller.signal,
        )
        .then((attachment) => {
          update(key, { progress: 1, attachment });
          return finish(key, attachment);
        })
        .catch((error: unknown) => {
          const cancelled =
            error instanceof DOMException && error.name === "AbortError";
          if (cancelled) {
            setUploads((current) =>
              current.filter((upload) => upload.key !== key),
            );
            return;
          }
          update(key, {
            status: "failed",
            error: failureMessage(error, UPLOAD_COPY.failed),
          });
        })
        .finally(() => controllers.current.delete(key));
    },
    [finish],
  );

  const add = useCallback(
    (files: FileList | File[], kind: AttachmentKind) =>
      Array.from(files).forEach((file) => start(file, kind)),
    [start],
  );
  const retry = useCallback(
    (upload: Upload) => {
      if (upload.status === "linkFailed" && upload.attachment) {
        finish(upload.key, upload.attachment);
        return;
      }
      start(upload.file, upload.kind, upload.key);
    },
    [start, finish],
  );
  const remove = useCallback((key: string) => {
    controllers.current.get(key)?.abort();
    setUploads((current) => current.filter((upload) => upload.key !== key));
  }, []);
  const clear = useCallback(() => setUploads([]), []);

  const attachmentIds = uploads.flatMap((upload) =>
    upload.status === "done" && upload.attachment ? [upload.attachment.id] : [],
  );
  const busy = uploads.some(
    (upload) => upload.status === "uploading" || upload.status === "linking",
  );
  return { uploads, add, retry, remove, clear, attachmentIds, busy };
}
