import type { ItemInput } from "../../api/types";
import {
  CAPTURE_LIFETIME_MS,
  CAPTURE_MAXIMUM_CHARACTERS,
} from "../../constants/limits";

// Storage can be unavailable (private mode, quota); a draft is a convenience.

export function readDraft(key: string): Partial<ItemInput> | null {
  try {
    const text = window.sessionStorage.getItem(key);
    return text ? (JSON.parse(text) as Partial<ItemInput>) : null;
  } catch {
    return null;
  }
}

export function writeDraft(key: string, input: ItemInput) {
  try {
    window.sessionStorage.setItem(key, JSON.stringify(input));
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
