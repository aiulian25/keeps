import { useId, useState, type FormEvent } from "react";
import { Copy } from "lucide-react";
import type { TotpSetup } from "../../api/types";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { AUTHENTICATOR_CODE_LENGTH, MANUAL_KEY_GROUP_SIZE } from "./constants";
import { AUTHENTICATOR_COPY } from "./copy";
import { fieldErrors, formErrorMessage } from "./errors";
import { useCopied } from "./useCopied";
import styles from "./Account.module.css";

const KEY_GROUPS = new RegExp(`.{1,${MANUAL_KEY_GROUP_SIZE}}`, "g");

interface AuthenticatorSetupDialogProps {
  setup: TotpSetup | null;
  onConfirm: (code: string) => Promise<void>;
  onClose: () => void;
}

/** QR code or manual key, then one code to prove the app is set up. */
export function AuthenticatorSetupDialog({
  setup,
  onConfirm,
  onClose,
}: AuthenticatorSetupDialogProps) {
  const formId = useId();
  const [code, setCode] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const { copied, copy } = useCopied();

  const close = () => {
    setCode("");
    setError(null);
    onClose();
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onConfirm(code.trim());
      setCode("");
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  const groupedKey = setup?.secret.match(KEY_GROUPS)?.join(" ") ?? "";
  return (
    <Dialog
      open={setup !== null}
      title={AUTHENTICATOR_COPY.setupTitle}
      onClose={close}
      footer={
        <>
          <Button onClick={close}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={AUTHENTICATOR_COPY.verifying}
          >
            {AUTHENTICATOR_COPY.verify}
          </Button>
        </>
      }
    >
      <p className="muted">{AUTHENTICATOR_COPY.scan}</p>
      {setup && (
        <img
          className={styles.qrCode}
          src={setup.qrCode}
          alt={AUTHENTICATOR_COPY.qrAlt}
        />
      )}
      <div className={styles.manualKey}>
        <span className={styles.rowLabel}>{AUTHENTICATOR_COPY.manualKey}</span>
        <code>{groupedKey}</code>
        <Button
          compact
          icon={<Copy aria-hidden="true" />}
          onClick={() => copy(setup?.secret ?? "")}
        >
          {copied ? AUTHENTICATOR_COPY.copied : AUTHENTICATOR_COPY.copyKey}
        </Button>
      </div>
      <form id={formId} className="stack" onSubmit={submit}>
        <FormError message={formErrorMessage(error, ["code"])} />
        <TextField
          label={AUTHENTICATOR_COPY.code}
          autoComplete="one-time-code"
          inputMode="numeric"
          maxLength={AUTHENTICATOR_CODE_LENGTH}
          value={code}
          onValueChange={setCode}
          error={fieldErrors(error).code}
          required
        />
      </form>
    </Dialog>
  );
}
