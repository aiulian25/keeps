import type { Attachment, ItemInput } from "../../api/types";
import {
  CAPTURE_LIFETIME_MS,
  CAPTURE_MAXIMUM_CHARACTERS,
} from "../../constants/limits";
import { MANUAL_DRAFT_VERSION } from "./copy";

// Storage can be unavailable (private mode, quota); a draft is a convenience.

/**
 * The manual form as it was last seen, so a refresh resumes it: the history
 * entry it belongs to, the typed values, files that finished uploading (by
 * reference; their bytes stay on the server), and the key that makes a
 * repeated save return the same item.
 */
export interface ManualDraft {
  entry: string;
  input: Partial<ItemInput>;
  idempotencyKey: string;
  uploads: Attachment[];
  unfinishedUploads: number;
  mainPhotoId: number | null;
}

const ATTACHMENT_KINDS: readonly string[] = ["photo", "invoice", "document"];

function isAttachment(value: unknown): value is Attachment {
  const candidate = value as Partial<Attachment> | null;
  return (
    typeof candidate?.id === "number" &&
    typeof candidate.filename === "string" &&
    ATTACHMENT_KINDS.includes(candidate.kind ?? "")
  );
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function draftFrom(stored: Record<string, unknown>): ManualDraft | null {
  if (!isObject(stored.input) || typeof stored.entry !== "string") {
    return null;
  }
  const uploads = Array.isArray(stored.uploads) ? stored.uploads : [];
  return {
    entry: stored.entry,
    input: stored.input as Partial<ItemInput>,
    idempotencyKey:
      typeof stored.idempotencyKey === "string" ? stored.idempotencyKey : "",
    uploads: uploads.filter(isAttachment),
    unfinishedUploads:
      typeof stored.unfinishedUploads === "number"
        ? stored.unfinishedUploads
        : 0,
    mainPhotoId:
      typeof stored.mainPhotoId === "number" ? stored.mainPhotoId : null,
  };
}

/** An older draft is just the field values, not tied to any history entry. */
function legacyDraft(stored: Record<string, unknown>): ManualDraft | null {
  if (typeof stored.name !== "string") {
    return null;
  }
  return {
    entry: "",
    input: stored as Partial<ItemInput>,
    idempotencyKey: "",
    uploads: [],
    unfinishedUploads: 0,
    mainPhotoId: null,
  };
}

/** Anything malformed or from another version counts as no draft. */
export function readManualDraft(key: string): ManualDraft | null {
  try {
    const text = window.sessionStorage.getItem(key);
    const stored: unknown = text ? JSON.parse(text) : null;
    if (!isObject(stored)) {
      return null;
    }
    if (stored.version === MANUAL_DRAFT_VERSION) {
      return draftFrom(stored);
    }
    return stored.version === undefined ? legacyDraft(stored) : null;
  } catch {
    return null;
  }
}

export function writeManualDraft(key: string, draft: ManualDraft) {
  try {
    window.sessionStorage.setItem(
      key,
      JSON.stringify({ version: MANUAL_DRAFT_VERSION, ...draft }),
    );
  } catch {
    // ignore: the form still holds the values in memory
  }
}

export function clearDraft(key: string) {
  try {
    window.sessionStorage.removeItem(key);
  } catch {
    // ignore
  }
}

// A capture from the Add to Keeps button is kept briefly, so a failed start
// can be retried or a refresh survives, without keeping page data for long.

export type CaptureRead<T> =
  { state: "none" } | { state: "expired" } | { state: "found"; value: T };

export function writeCapture(key: string, value: unknown) {
  try {
    const text = JSON.stringify({ savedAt: Date.now(), value });
    if (text.length <= CAPTURE_MAXIMUM_CHARACTERS) {
      window.sessionStorage.setItem(key, text);
    }
  } catch {
    // ignore: the capture is still in memory for this page
  }
}

/** Reads a kept capture; anything malformed, too large or invalid counts as none. */
export function readCapture<T>(
  key: string,
  validate: (value: unknown) => T | null,
): CaptureRead<T> {
  try {
    const text = window.sessionStorage.getItem(key);
    if (!text || text.length > CAPTURE_MAXIMUM_CHARACTERS) {
      return { state: "none" };
    }
    const stored = JSON.parse(text) as { savedAt?: unknown; value?: unknown };
    if (typeof stored.savedAt !== "number") {
      return { state: "none" };
    }
    if (Date.now() - stored.savedAt > CAPTURE_LIFETIME_MS) {
      window.sessionStorage.removeItem(key);
      return { state: "expired" };
    }
    const value = validate(stored.value);
    return value === null ? { state: "none" } : { state: "found", value };
  } catch {
    return { state: "none" };
  }
}
