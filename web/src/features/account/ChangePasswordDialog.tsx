import { useId, useState, type FormEvent } from "react";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, PasswordField } from "../../components/form/Field";
import { useToast } from "../../components/Toast";
import { COMMON } from "../../constants/copy";
import { PASSWORD_MINIMUM_LENGTH } from "../../constants/limits";
import { PASSWORD_COPY } from "./copy";
import { fieldErrors, formErrorMessage } from "./errors";
import {
  EMPTY_SECOND_FACTOR,
  SecondFactorField,
  secondFactorBody,
} from "./SecondFactorField";
import { useAccountActions } from "./useAccountActions";

interface ChangePasswordDialogProps {
  open: boolean;
  needsSecondFactor: boolean;
  onClose: () => void;
}

export function ChangePasswordDialog({
  open,
  needsSecondFactor,
  onClose,
}: ChangePasswordDialogProps) {
  const formId = useId();
  const { changePassword } = useAccountActions();
  const { notify } = useToast();
  const [password, setPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [secondFactor, setSecondFactor] = useState(EMPTY_SECOND_FACTOR);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const close = () => {
    setPassword("");
    setNewPassword("");
    setSecondFactor(EMPTY_SECOND_FACTOR);
    setError(null);
    onClose();
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await changePassword({
        password,
        newPassword,
        ...secondFactorBody(secondFactor),
      });
      notify({
        title: PASSWORD_COPY.changed,
        detail: PASSWORD_COPY.changedDetail,
      });
      close();
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  const fields = fieldErrors(error);
  return (
    <Dialog
      open={open}
      title={PASSWORD_COPY.title}
      onClose={close}
      footer={
        <>
          <Button onClick={close}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            busy={busy}
            busyLabel={PASSWORD_COPY.submitting}
          >
            {PASSWORD_COPY.submit}
          </Button>
        </>
      }
    >
      <form id={formId} className="stack" onSubmit={submit}>
        <FormError
          message={formErrorMessage(error, [
            "password",
            "newPassword",
            "code",
            "recoveryCode",
          ])}
        />
        <PasswordField
          label={PASSWORD_COPY.current}
          autoComplete="current-password"
          value={password}
          onValueChange={setPassword}
          error={fields.password}
          required
          autoFocus
        />
        <PasswordField
          label={PASSWORD_COPY.newPassword}
          autoComplete="new-password"
          hint={PASSWORD_COPY.newPasswordHint}
          minLength={PASSWORD_MINIMUM_LENGTH}
          value={newPassword}
          onValueChange={setNewPassword}
          error={fields.newPassword}
          required
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
