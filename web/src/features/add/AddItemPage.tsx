import { useEffect, useRef, useState, type FormEvent } from "react";
import { ArrowRight, Box, Link2, Pencil, Upload } from "lucide-react";
import { useLocation, useNavigate, Link } from "react-router";
import { ApiError } from "../../api/client";
import { importsApi } from "../../api/endpoints";
import { Button } from "../../components/Button";
import { FieldShell } from "../../components/form/Field";
import fieldStyles from "../../components/form/Field.module.css";
import { COMMON, ERRORS } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import {
  ADD_COPY,
  LINK_ERRORS,
  PRIVATE_DOCUMENT_CODE,
  BUTTON_COPY,
} from "./copy";
import type { AddItemState, ManualItemState } from "./manualState";
import { useGoBack } from "../../hooks/useGoBack";
import styles from "./Add.module.css";

function linkErrorFor(error: unknown) {
  if (!(error instanceof ApiError)) {
    return { message: ERRORS.generic, code: "" };
  }
  return {
    message: LINK_ERRORS[error.code] ?? error.fields.url ?? error.message,
    code: error.code,
  };
}

/** Panel 04: one link field, one primary action, two ways round it. */
export function AddItemPage() {
  const navigate = useNavigate();
  const incoming = (useLocation().state ?? {}) as AddItemState;
  const room = incoming.room ?? "";
  const [url, setUrl] = useState(incoming.url ?? "");
  const [starting, setStarting] = useState(false);
  const [linkError, setLinkError] = useState({ message: "", code: "" });
  const autoStarted = useRef(false);
  const trimmedUrl = url.trim();

  const start = async (link: string) => {
    setStarting(true);
    setLinkError({ message: "", code: "" });
    try {
      const draft = await importsApi.start(link);
      const state: AddItemState = { url: link, room };
      navigate(ROUTES.addImport(draft.id), { replace: true, state });
    } catch (error) {
      setLinkError(linkErrorFor(error));
      setStarting(false);
    }
  };

  useEffect(() => {
    const link = incoming.url?.trim() ?? "";
    const shouldAutoStart =
      incoming.autoStart && link !== "" && !autoStarted.current;
    if (!shouldAutoStart) {
      return;
    }
    autoStarted.current = true;
    start(link);
    // Runs once for the link Home handed over.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (trimmedUrl === "") {
      return;
    }
    start(trimmedUrl);
  };

  const leave = useGoBack(ROUTES.items);

  const goManual = (focusInvoice: boolean) => {
    const state: ManualItemState = {
      url: trimmedUrl || undefined,
      focusInvoice,
      fields: { room },
    };
    navigate(ROUTES.addManual, { state });
  };

  return (
    <div className={styles.entry}>
      <div className={styles.entryHeading}>
        <Box className={styles.headingIcon} aria-hidden="true" />
        <h1>{ADD_COPY.title}</h1>
        <p className="muted">{ADD_COPY.subtitle}</p>
      </div>
      <form className={styles.entryForm} onSubmit={submit} noValidate>
        <FieldShell
          label={ADD_COPY.urlLabel}
          className={styles.urlField}
          error={linkError.message || undefined}
        >
          {({ id, describedBy, invalid }) => (
            <div
              className={fieldStyles.control}
              data-invalid={invalid || undefined}
            >
              <span className={fieldStyles.prefix}>
                <Link2 aria-hidden="true" />
              </span>
              <input
                id={id}
                className={fieldStyles.input}
                type="url"
                inputMode="url"
                autoComplete="off"
                placeholder={ADD_COPY.urlPlaceholder}
                value={url}
                onChange={(event) => setUrl(event.target.value)}
                aria-describedby={describedBy}
                aria-invalid={invalid || undefined}
              />
            </div>
          )}
        </FieldShell>
        {linkError.code === PRIVATE_DOCUMENT_CODE && (
          <Button
            compact
            icon={<Upload aria-hidden="true" />}
            onClick={() => goManual(true)}
            className={styles.inlineAction}
          >
            {ADD_COPY.uploadReceipt}
          </Button>
        )}
        <Button
          type="submit"
          variant="primary"
          wide
          disabled={trimmedUrl === ""}
          busy={starting}
          busyLabel={ADD_COPY.starting}
          iconAfter={<ArrowRight aria-hidden="true" />}
        >
          {ADD_COPY.getDetails}
        </Button>
      </form>
      <div className={styles.divider}>{COMMON.or}</div>
      <div className={styles.alternatives}>
        <Button
          icon={<Pencil aria-hidden="true" />}
          onClick={() => goManual(false)}
        >
          {ADD_COPY.addManually}
        </Button>
        <Button
          icon={<Upload aria-hidden="true" />}
          onClick={() => goManual(true)}
        >
          {ADD_COPY.uploadReceipt}
        </Button>
      </div>
      <Link to={ROUTES.addButton} className={styles.buttonLink}>
        {BUTTON_COPY.shopBlocks}
      </Link>
      <Button variant="ghost" className={styles.cancelButton} onClick={leave}>
        {COMMON.cancel}
      </Button>
    </div>
  );
}
