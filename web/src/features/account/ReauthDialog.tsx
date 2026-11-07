import { useId, useState, type FormEvent } from "react";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, PasswordField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { REAUTH_COPY } from "./copy";
import { fieldErrors, formErrorMessage } from "./errors";
import {
  EMPTY_SECOND_FACTOR,
  SecondFactorField,
  secondFactorBody,
} from "./SecondFactorField";

export interface ReauthCredentials {
  password: string;
  code?: string;
  recoveryCode?: string;
}

interface ReauthDialogProps {
  open: boolean;
  needsSecondFactor: boolean;
  onConfirm: (credentials: ReauthCredentials) => Promise<void>;
  onCancel: () => void;
}

/** Password (plus authenticator code when enabled) before sensitive changes. */
export function ReauthDialog({
  open,
  needsSecondFactor,
  onConfirm,
  onCancel,
}: ReauthDialogProps) {
  const formId = useId();
  const [password, setPassword] = useState("");
  const [secondFactor, setSecondFactor] = useState(EMPTY_SECOND_FACTOR);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const reset = () => {
    setPassword("");
    setSecondFactor(EMPTY_SECOND_FACTOR);
    setError(null);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onConfirm({ password, ...secondFactorBody(secondFactor) });
      reset();
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  const cancel = () => {
    reset();
    onCancel();
  };

  const fields = fieldErrors(error);
  return (
    <Dialog
      open={open}
      title={REAUTH_COPY.title}
      onClose={cancel}
      footer={
        <>
          <Button onClick={cancel}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={REAUTH_COPY.submitting}
          >
            {REAUTH_COPY.submit}
          </Button>
        </>
      }
    >
      <form id={formId} className="stack" onSubmit={submit}>
        <p className="muted">{REAUTH_COPY.hint}</p>
        <FormError
          message={formErrorMessage(error, [
            "password",
            "code",
            "recoveryCode",
          ])}
        />
        <PasswordField
          label={REAUTH_COPY.password}
          autoComplete="current-password"
          value={password}
          onValueChange={setPassword}
          error={fields.password}
          required
          autoFocus
        />
        {needsSecondFactor && (
          <SecondFactorField
            value={secondFactor}
            onChange={setSecondFactor}
            error={fields.code ?? fields.recoveryCode}
          />
        )}
      </form>
    </Dialog>
  );
}
