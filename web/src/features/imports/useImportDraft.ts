import { useCallback, useEffect, useState } from "react";
import { ApiError, STATUS_NOT_FOUND } from "../../api/client";
import { importsApi } from "../../api/endpoints";
import type { ImportDraft, ImportState } from "../../api/types";
import { ERRORS } from "../../constants/copy";
import { IMPORT_POLL_MS } from "../../constants/limits";

/** The server stops a job after 20 s; a little slack covers queueing. */
const CLIENT_TIMEOUT_MS = 25_000;
const FINISHED_STATES: ImportState[] = ["done", "failed", "cancelled", "saved"];

function isAbort(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

/**
 * Polls one import until it finishes, fails, or this wait runs out. A failed
 * read or a long wait doesn't end the job on the server: `checkAgain` polls
 * the same import again. `expired` means the server no longer has it.
 */
export function useImportDraft(importId: string) {
  const [draft, setDraft] = useState<ImportDraft | null>(null);
  const [timedOut, setTimedOut] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [expired, setExpired] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const startedAt = Date.now();
    let timer = 0;
    const poll = async () => {
      try {
        const next = await importsApi.get(importId, controller.signal);
        setDraft(next);
        if (FINISHED_STATES.includes(next.state)) {
          return;
        }
      } catch (error) {
        if (isAbort(error)) {
          return;
        }
        setExpired(
          error instanceof ApiError && error.status === STATUS_NOT_FOUND,
        );
        setLoadError(
          error instanceof ApiError ? error.message : ERRORS.generic,
        );
        return;
      }
      if (Date.now() - startedAt > CLIENT_TIMEOUT_MS) {
        setTimedOut(true);
        return;
      }
      timer = window.setTimeout(poll, IMPORT_POLL_MS);
    };
    poll();
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [importId, attempt]);

  const checkAgain = useCallback(() => {
    setTimedOut(false);
    setLoadError("");
    setExpired(false);
    setAttempt((count) => count + 1);
  }, []);

  return { draft, timedOut, loadError, expired, checkAgain };
}
