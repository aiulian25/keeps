import { Navigate, useLocation, useNavigate, useParams } from "react-router";
import { importsApi } from "../../api/endpoints";
import type { ItemInput } from "../../api/types";
import { ROUTES } from "../../constants/routes";
import type { AddItemState, ManualItemState } from "../add/manualState";
import { candidatesOf, fieldsFromCandidate } from "./candidateInput";
import { CLIENT_TIMEOUT_CODE, RETRYABLE_CODES } from "./copy";
import { ImportProblem, ImportProgress } from "./ImportStatus";
import { ReviewImport } from "./ReviewImport";
import { useImportDraft } from "./useImportDraft";

function problemCodeFor(
  failed: boolean,
  errorCode: string | undefined,
  loadError: string,
) {
  if (loadError) {
    return "";
  }
  if (failed) {
    return errorCode ?? "";
  }
  return CLIENT_TIMEOUT_CODE;
}

export function ImportPage() {
  const { importId = "" } = useParams();
  return <ImportFlow key={importId} importId={importId} />;
}

/** Progress → review, or the problem screen; leaving never loses the link. */
function ImportFlow({ importId }: { importId: string }) {
  const navigate = useNavigate();
  const handedUrl = ((useLocation().state ?? {}) as AddItemState).url ?? "";
  const { draft, timedOut, loadError } = useImportDraft(importId);
  const sourceUrl = draft?.sourceUrl || handedUrl;

  const goManual = (focusInvoice: boolean, fields?: Partial<ItemInput>) => {
    const state: ManualItemState = {
      url: sourceUrl || undefined,
      focusInvoice,
      fields,
    };
    navigate(ROUTES.addManual, { state });
  };
  const cancelImport = () => importsApi.cancel(importId).catch(() => undefined);
  const retry = () => {
    const state: AddItemState = { url: sourceUrl, autoStart: true };
    navigate(ROUTES.add, { replace: true, state });
  };

  if (draft?.state === "saved") {
    return <Navigate to={ROUTES.items} replace />;
  }
  if (draft?.state === "cancelled") {
    return <Navigate to={ROUTES.add} replace state={{ url: sourceUrl }} />;
  }
  if (draft?.state === "done") {
    return (
      <ReviewImport
        draft={draft}
        onAddManually={(fields) => goManual(false, fields)}
      />
    );
  }

  const failed = draft?.state === "failed";
  const stillRunning = !failed && !timedOut && loadError === "";
  if (stillRunning) {
    return (
      <ImportProgress
        host={draft?.sourceHost ?? ""}
        onCancel={() => {
          cancelImport();
          navigate(ROUTES.add, { replace: true, state: { url: sourceUrl } });
        }}
        onAddManually={() => {
          cancelImport();
          goManual(false);
        }}
      />
    );
  }

  const code = problemCodeFor(failed, draft?.errorCode, loadError);
  const partialFields =
    draft && draft.candidates.length > 0
      ? fieldsFromCandidate(candidatesOf(draft)[0])
      : undefined;
  const canRetry =
    sourceUrl !== "" && (RETRYABLE_CODES.includes(code) || loadError !== "");
  return (
    <ImportProblem
      code={code}
      reason={loadError}
      onAddManually={() => goManual(false, partialFields)}
      onUploadReceipt={() => goManual(true, partialFields)}
      onRetry={canRetry ? retry : undefined}
    />
  );
}
