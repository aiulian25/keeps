import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { ApiError } from "../../api/client";
import { importsApi } from "../../api/endpoints";
import type { PageSnapshot } from "../../api/types";
import { Button } from "../../components/Button";
import { LoadingLine } from "../../components/Bits";
import { FormError } from "../../components/form/Field";
import { PageHeader } from "../../components/shell/PageHeader";
import { ROUTES } from "../../constants/routes";
import { decodeSnapshot } from "./bookmarklet";
import { BUTTON_COPY, CAPTURE_STORAGE_KEY } from "./copy";
import type { ManualItemState } from "./manualState";
import { clearDraft, readCapture, writeCapture } from "./sessionDraft";
import styles from "./Add.module.css";

function asSnapshot(value: unknown): PageSnapshot | null {
  const candidate = value as Partial<PageSnapshot> | null;
  const valid =
    candidate !== null &&
    typeof candidate === "object" &&
    typeof candidate.url === "string" &&
    typeof candidate.title === "string" &&
    Array.isArray(candidate.metaTags) &&
    Array.isArray(candidate.structuredData);
  return valid ? (candidate as PageSnapshot) : null;
}

function readFragment(): PageSnapshot | null {
  try {
    return asSnapshot(decodeSnapshot(window.location.hash.slice(1)));
  } catch {
    return null;
  }
}

interface Capture {
  snapshot: PageSnapshot | null;
  problem: string;
}

/** The page details from the URL, or the ones kept from a moment ago. */
function initialCapture(): Capture {
  const fromFragment = readFragment();
  if (fromFragment) {
    writeCapture(CAPTURE_STORAGE_KEY, fromFragment);
    return { snapshot: fromFragment, problem: "" };
  }
  const kept = readCapture(CAPTURE_STORAGE_KEY, asSnapshot);
  if (kept.state === "found") {
    return { snapshot: kept.value, problem: "" };
  }
  const problem =
    kept.state === "expired" ? BUTTON_COPY.expired : BUTTON_COPY.unreadable;
  return { snapshot: null, problem };
}

/**
 * Receives page details from the Add to Keeps button and opens the review.
 * The URL fragment is cleared at once; a short-lived copy lets a failed start
 * be retried, or handed to manual entry, without using the button again.
 */
export function FromPagePage() {
  const navigate = useNavigate();
  const [{ snapshot, problem }] = useState(initialCapture);
  const [error, setError] = useState(problem);
  const [busy, setBusy] = useState(false);
  const started = useRef(false);

  const start = useCallback(() => {
    if (snapshot === null) {
      return;
    }
    setBusy(true);
    setError("");
    importsApi
      .startFromPage(snapshot)
      .then((draft) => {
        clearDraft(CAPTURE_STORAGE_KEY);
        navigate(ROUTES.addImport(draft.id), { replace: true });
      })
      .catch((caught: unknown) =>
        setError(
          caught instanceof ApiError ? caught.message : BUTTON_COPY.unreadable,
        ),
      )
      .finally(() => setBusy(false));
  }, [navigate, snapshot]);

  useEffect(() => {
    window.history.replaceState(null, "", window.location.pathname);
    if (started.current) {
      return;
    }
    started.current = true;
    start();
  }, [start]);

  const enterManually = () => {
    clearDraft(CAPTURE_STORAGE_KEY);
    const state: ManualItemState = snapshot
      ? { url: snapshot.url, fields: { name: snapshot.title } }
      : {};
    navigate(ROUTES.addManual, { state });
  };
  const discard = () => {
    clearDraft(CAPTURE_STORAGE_KEY);
    navigate(ROUTES.add);
  };

  if (!error) {
    return <LoadingLine text={BUTTON_COPY.receiving} />;
  }
  return (
    <div className="stack">
      <PageHeader title={BUTTON_COPY.title} backTo={ROUTES.add} />
      <FormError message={error} />
      <div className={styles.actions}>
        {snapshot && (
          <Button variant="primary" busy={busy} onClick={start}>
            {BUTTON_COPY.retry}
          </Button>
        )}
        <Button onClick={enterManually}>{BUTTON_COPY.enterManually}</Button>
        {snapshot && (
          <Button variant="ghost" onClick={discard}>
            {BUTTON_COPY.cancel}
          </Button>
        )}
      </div>
    </div>
  );
}
