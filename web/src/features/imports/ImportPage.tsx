import { Navigate, useLocation, useNavigate, useParams } from "react-router";
import { importsApi } from "../../api/endpoints";
import type { ImportDraft } from "../../api/types";
import type { ItemInput } from "../../api/types";
import { ROUTES } from "../../constants/routes";
import type { AddItemState, ManualItemState } from "../add/manualState";
import { candidatesOf, fieldsFromCandidate } from "./candidateInput";
import { IMPORT_COPY, RETRYABLE_CODES, STILL_WAITING_CODE } from "./copy";
import { ImportProblem, ImportProgress } from "./ImportStatus";
import { ReviewImport } from "./ReviewImport";
import { useImportDraft } from "./useImportDraft";

function partialFieldsOf(draft: ImportDraft | null) {
  if (!draft || draft.candidates.length === 0) {
    return undefined;
  }
  return fieldsFromCandidate(candidatesOf(draft)[0]);
}

export function ImportPage() {
  const { importId = "" } = useParams();
  return <ImportFlow key={importId} importId={importId} />;
}

/** Progress → review, or the problem screen; leaving never loses the link. */
function ImportFlow({ importId }: { importId: string }) {
  const navigate = useNavigate();
  const handedUrl = ((useLocation().state ?? {}) as AddItemState).url ?? "";
  const { draft, timedOut, loadError, expired, checkAgain } =
    useImportDraft(importId);
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

  const partialFields = partialFieldsOf(draft);
  const addManually = () => goManual(false, partialFields);
  const uploadReceipt = () => goManual(true, partialFields);
  // Not known to have ended: look at the same job again instead of starting
  // another, which would fetch the shop page a second time.
  const mayStillRun = !failed && !expired;
  if (mayStillRun) {
    return (
      <ImportProblem
        code={STILL_WAITING_CODE}
        reason={loadError}
        onAddManually={addManually}
        onUploadReceipt={uploadReceipt}
        onRetry={checkAgain}
        retryLabel={IMPORT_COPY.checkAgain}
      />
    );
  }

  const code = expired ? "" : (draft?.errorCode ?? "");
  const canRetry =
    sourceUrl !== "" && (expired || RETRYABLE_CODES.includes(code));
  return (
    <ImportProblem
      code={code}
      reason={loadError}
      onAddManually={addManually}
      onUploadReceipt={uploadReceipt}
      onRetry={canRetry ? retry : undefined}
    />
  );
}
